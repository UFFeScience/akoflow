import copy
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class NativeMaterializationTest(unittest.TestCase):
    image = os.environ.get("SIMGRID_TEST_IMAGE", "akoflow-simgrid-ako141-build")
    root = Path(__file__).resolve().parent

    def run_model(self, model, platform=None, success=True):
        with tempfile.TemporaryDirectory(prefix="akoflow-native-test-") as directory:
            case = Path(directory)
            (case / "input.json").write_text(json.dumps(model))
            (case / "platform.xml").write_text(platform or (self.root / "materialization-platform.xml").read_text())
            command = ["docker", "run", "--rm", "-v", f"{directory}:/case", self.image,
                       "/build/output/akoflow-simgrid-runner", "--platform", "/case/platform.xml",
                       "--input", "/case/input.json", "--output", "/case/output.json"]
            result = subprocess.run(command, capture_output=True, text=True, timeout=30)
            if not success:
                self.assertNotEqual(result.returncode, 0)
                return result.stderr
            self.assertEqual(result.returncode, 0, result.stderr)
            return json.loads((case / "output.json").read_text())

    def fixture(self):
        return json.loads((self.root / "materialization-simulation.json").read_text())

    def test_mixed_origins_serial_relay_and_single_consumer_merge(self):
        result = self.run_model(self.fixture())
        tasks = {task["activityId"]: task for task in result["tasks"]}
        transfers = {flow["producerActivityId"]: flow for flow in result["transfers"]}
        last_producer = max(tasks["a"]["finishedAt"], tasks["c"]["finishedAt"])
        # Both routes wait for all producers, not only their own source.
        self.assertAlmostEqual(transfers["a"]["startedAt"], last_producer, places=6)
        self.assertAlmostEqual(transfers["c"]["startedAt"], last_producer, places=6)
        phases = transfers["c"]["route"]["phases"]
        self.assertEqual(len(phases), 2)
        self.assertAlmostEqual(phases[0]["finishedAt"], phases[1]["startedAt"], places=6)
        self.assertGreaterEqual(phases[0]["durationSeconds"], 1.999)
        self.assertGreaterEqual(phases[1]["durationSeconds"], 2.999)
        prepared = tasks["b"]["metadata"]
        self.assertAlmostEqual(prepared["workspacePreparationSeconds"], 2)
        self.assertAlmostEqual(prepared["workspacePreparationStartedAt"], transfers["c"]["finishedAt"], places=6)
        self.assertAlmostEqual(tasks["b"]["startedAt"], transfers["c"]["finishedAt"] + 2, places=6)
        self.assertAlmostEqual(result["executed"]["makespanSeconds"], 13, places=5)
        self.assertEqual(transfers["c"]["networkBytes"], 200)
        self.assertEqual(transfers["c"]["route"]["serverNetworkBytes"], 200)
        self.assertEqual(transfers["a"]["route"]["serverNetworkBytes"], 0)

    def test_shared_link_contention_is_not_ignored(self):
        model = self.fixture()
        model["tasks"][0]["resourceId"] = "hpc"
        model["tasks"][0]["flops"] = 1000000000
        model["tasks"][1]["flops"] = 1000000000
        second = copy.deepcopy(model["dependencies"][1])
        second["producerId"] = "a"
        for flow in [second, model["dependencies"][1]]:
            for stage in flow["stages"]:
                stage["effectiveBytesPerSecond"] = 1000
                stage["afterSeconds"] = 0
        model["dependencies"][0] = second
        platform = (self.root / "materialization-platform.xml").read_text().replace('bandwidth="1000000000Bps"', 'bandwidth="100Bps"')
        result = self.run_model(model, platform)
        for flow in result["transfers"]:
            self.assertGreater(flow["durationSeconds"], 3.9)  # 200B ingress then 200B egress sharing.

    def test_shared_service_slot_serializes_distinct_routes_and_merge(self):
        model = self.fixture()
        # Both routes and the final merge use one control transport slot.
        # There is no network bottleneck in this fixture.
        for flow in model["dependencies"]:
            for stage in flow["stages"]:
                stage["serviceGroupId"] = "control-transport"
                stage["serviceSlots"] = 1
        consumer = next(task for task in model["tasks"] if task["id"] == "b")
        consumer["preparationServiceGroupId"] = "control-transport"
        consumer["preparationServiceSlots"] = 1
        result = self.run_model(model)
        # roots 5 + peer 1 + relay 2+3 + merge 2 + compute 1.
        self.assertAlmostEqual(result["executed"]["makespanSeconds"], 14, places=5)
        model["dependencies"][0]["stages"][0]["serviceSlots"] = 2
        self.assertIn("conflicting shared service capacities", self.run_model(model, success=False))

    def test_six_predecessors_use_two_waves_of_three_transport_slots(self):
        model = self.fixture()
        model["tasks"] = [{"id": f"p{i}", "assignmentId": f"p{i}",
                           "resourceId": "hpc", "flops": 1000000000} for i in range(6)]
        model["resourceLaneOrders"] = [{"predecessorId": f"p{i-1}", "successorId": f"p{i}"} for i in range(1, 6)]
        model["tasks"].append({"id": "b", "assignmentId": "b", "resourceId": "cloud-b",
                               "flops": 1000000000, "preparationSeconds": 2,
                               "preparationServiceGroupId": "cloud-session", "preparationServiceSlots": 3})
        model["dependencies"] = [{"producerId": f"p{i}", "consumerId": "b", "bytes": 100,
            "networkBytes": 0, "relayNetworkBytes": 0, "strategy": "gateway", "stages": [
                {"name": "source-pack", "sourceResourceId": "hpc", "targetResourceId": "relay",
                 "bytes": 0, "beforeSeconds": 10, "serviceGroupId": "hpc-session", "serviceSlots": 3},
                {"name": "target-extract", "sourceResourceId": "relay", "targetResourceId": "cloud-b",
                 "bytes": 0, "afterSeconds": 1, "serviceGroupId": "cloud-session", "serviceSlots": 3}
            ]} for i in range(6)]
        result = self.run_model(model)
        starts = sorted(flow["route"]["phases"][0]["startedAt"] for flow in result["transfers"])
        self.assertEqual(starts, [6, 6, 6, 16, 16, 16])
        waits = sorted(flow["route"]["phases"][0]["serviceWaitSeconds"] for flow in result["transfers"])
        self.assertEqual(waits, [0, 0, 0, 10, 10, 10])
        self.assertAlmostEqual(result["executed"]["makespanSeconds"], 30, places=5)

    def test_legacy_zero_byte_control_dependency_does_not_deadlock(self):
        model = self.fixture()
        model.pop("materializationModelVersion")
        for flow in model["dependencies"]:
            flow.pop("stages")
            flow["bytes"] = 0
        result = self.run_model(model)
        self.assertEqual(len(result["transfers"]), 0)
        self.assertEqual(len(result["tasks"]), 3)

    def test_proven_shared_storage_stage_does_not_move_network_bytes(self):
        model = self.fixture()
        model["tasks"] = [model["tasks"][1], model["tasks"][2]]
        model["tasks"][1]["resourceId"] = "hpc-b"
        model["tasks"][1]["preparationSeconds"] = 0
        flow = model["dependencies"][1]
        flow.update(strategy="shared-storage", networkBytes=0, relayNetworkBytes=0)
        flow["stages"] = [{"name":"shared-copy", "sourceResourceId":"hpc", "targetResourceId":"hpc-b",
                           "bytes":0, "beforeSeconds":1, "afterSeconds":2}]
        model["dependencies"] = [flow]
        platform = (self.root / "materialization-platform.xml").read_text().replace(
            '<host id="hpc"', '<host id="hpc-b" speed="1000000000f" core="1"/>\n    <host id="hpc"')
        result = self.run_model(model, platform)
        self.assertEqual(result["transfers"][0]["networkBytes"], 0)
        self.assertEqual(result["transfers"][0]["route"]["serverNetworkBytes"], 0)
        self.assertAlmostEqual(result["transfers"][0]["durationSeconds"], 3, places=6)

    def test_legacy_native_smoke_remains_supported(self):
        model = json.loads((self.root / "simulation.json").read_text())
        result = self.run_model(model, (self.root / "platform.xml").read_text())
        self.assertEqual(len(result["tasks"]), 2)
        self.assertEqual(len(result["transfers"]), 1)
        self.assertTrue(all(task["status"] == "completed" for task in result["tasks"]))

    def test_missing_stage_fails_instead_of_silently_reverting_to_edge_model(self):
        model = self.fixture()
        model["dependencies"][1].pop("stages")
        self.assertIn("lacks stages", self.run_model(model, success=False))

    def test_relay_strategy_cannot_hide_a_direct_single_hop(self):
        model = self.fixture()
        flow = model["dependencies"][1]
        flow["stages"] = [{"name":"fake-relay", "sourceResourceId":"hpc", "targetResourceId":"cloud-b",
                           "bytes":200, "effectiveBytesPerSecond":100}]
        self.assertIn("must use both relay hops", self.run_model(model, success=False))

    def test_runner_advertises_materialization_version(self):
        result = subprocess.run(["docker", "run", "--rm", self.image,
                                 "/build/output/akoflow-simgrid-runner", "--capabilities"],
                                capture_output=True, text=True, timeout=30, check=True)
        self.assertEqual(json.loads(result.stdout)["workspaceMaterializationModelVersion"], 1)


if __name__ == "__main__":
    unittest.main()
