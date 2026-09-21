import unittest

from derive_sla_thresholds import derive_thresholds
from launch_campaign_simulations import choose_candidate


class CandidateSelectionTests(unittest.TestCase):
    def test_time_and_cost_choose_different_candidates(self):
        candidates = [
            {
                "id": "fast",
                "algorithm": "prism-time",
                "feasible": True,
                "rank": 2,
                "predicted": {"makespanSeconds": 10, "cost": 5},
            },
            {
                "id": "slow",
                "algorithm": "prism-time",
                "feasible": True,
                "rank": 1,
                "predicted": {"makespanSeconds": 20, "cost": 1},
            },
            {
                "id": "cheap",
                "algorithm": "prism-cost",
                "feasible": True,
                "rank": 2,
                "predicted": {"makespanSeconds": 20, "cost": 1},
            },
            {
                "id": "expensive",
                "algorithm": "prism-cost",
                "feasible": True,
                "rank": 1,
                "predicted": {"makespanSeconds": 10, "cost": 5},
            },
        ]
        self.assertEqual(choose_candidate(candidates, "prism-time")["id"], "fast")
        self.assertEqual(choose_candidate(candidates, "prism-cost")["id"], "cheap")

    def test_infeasible_candidate_is_never_selected(self):
        candidates = [
            {
                "id": "invalid",
                "algorithm": "heft",
                "feasible": False,
                "predicted": {"makespanSeconds": 1, "cost": 1},
            },
            {
                "id": "valid",
                "algorithm": "heft",
                "feasible": True,
                "predicted": {"makespanSeconds": 2, "cost": 2},
            },
        ]
        self.assertEqual(choose_candidate(candidates, "heft")["id"], "valid")


class SLAThresholdTests(unittest.TestCase):
    def test_thresholds_use_common_simulator_heft_reference(self):
        results = {
            "records": [
                {
                    "executionRunId": "heft-hybrid",
                    "workflowVersionId": "workflow-v1",
                    "executionScopeId": "hybrid",
                    "algorithm": "heft",
                    "status": "completed",
                    "observedMakespanSeconds": 100,
                    "observedCost": 10,
                },
                {
                    "executionRunId": "prism-hybrid",
                    "workflowVersionId": "workflow-v1",
                    "executionScopeId": "hybrid",
                    "algorithm": "prism-time",
                    "status": "completed",
                    "observedMakespanSeconds": 50,
                    "observedCost": 5,
                },
            ]
        }
        thresholds = derive_thresholds(results, "hybrid", [1.1, 1.2])
        self.assertEqual(len(thresholds), 2)
        self.assertAlmostEqual(thresholds[0]["deadlineSeconds"], 110)
        self.assertAlmostEqual(thresholds[0]["budget"], 11)
        self.assertAlmostEqual(thresholds[1]["deadlineSeconds"], 120)
        self.assertAlmostEqual(thresholds[1]["budget"], 12)

    def test_zero_cost_reference_is_rejected(self):
        results = {
            "records": [
                {
                    "executionRunId": "heft-hybrid",
                    "workflowVersionId": "workflow-v1",
                    "executionScopeId": "hybrid",
                    "algorithm": "heft",
                    "status": "completed",
                    "observedMakespanSeconds": 100,
                    "observedCost": 0,
                }
            ]
        }
        with self.assertRaisesRegex(RuntimeError, "zero would disable budget"):
            derive_thresholds(results, "hybrid", [1.2])


if __name__ == "__main__":
    unittest.main()
