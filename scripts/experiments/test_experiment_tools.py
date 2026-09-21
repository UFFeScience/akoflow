import unittest
from unittest.mock import patch
import json
import os
import sys
import tempfile
from pathlib import Path

from derive_sla_thresholds import derive_thresholds
from create_sla_campaign import build_session_definitions
from create_beam_campaign import build_beam_sessions
from create_interference_campaign import (
    build_interference_sessions,
    selected_activity_ids,
)
from create_reference_campaign import build_reference_sessions
from launch_campaign_simulations import choose_candidate, run_id, session_algorithms


class SimulationCollectionTests(unittest.TestCase):
    def test_null_detail_collections_are_treated_as_empty(self):
        from collect_simulation_results import main

        with tempfile.TemporaryDirectory() as directory:
            directory_path = Path(directory)
            manifest = directory_path / "manifest.json"
            output = directory_path / "results.json"
            manifest.write_text(
                json.dumps(
                    {
                        "records": [
                            {
                                "executionRunId": "run-1",
                                "predicted": {},
                            }
                        ]
                    }
                )
            )
            detail = {
                "run": {"id": "run-1", "status": "completed"},
                "activities": None,
                "dataTransfers": None,
                "handles": None,
                "events": None,
            }
            arguments = [
                "collect_simulation_results.py",
                "--manifest",
                str(manifest),
                "--output",
                str(output),
            ]
            with (
                patch.dict(os.environ, {"AKOFLOW_API_TOKEN": "test"}),
                patch("collect_simulation_results.get_json", return_value=detail),
                patch.object(sys, "argv", arguments),
            ):
                main()
            result = json.loads(output.read_text())
            self.assertEqual(result["statusCounts"], {"completed": 1})
            self.assertEqual(result["activities"], [])


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

    def test_algorithms_and_session_run_ids_follow_session_definition(self):
        session = {
            "id": "experiment-s001-c010-workflow-v1",
            "workflowVersionId": "workflow-v1",
            "executionScopeId": "scheduler-hybrid_hetero-scope-v1",
            "algorithms": [
                {"id": "prism-time"},
                {"id": "prism-cost"},
                {"id": "heft"},
            ],
        }
        self.assertEqual(
            session_algorithms(session),
            ["prism-time", "prism-cost", "heft"],
        )
        self.assertEqual(
            run_id("simulation", session, "heft", "session"),
            "simulation-experiment-s001-c010-workflow-v1-heft",
        )


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

    def test_zero_cost_uses_explicit_cloud_fallback(self):
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
                },
                {
                    "executionRunId": "heft-cloud",
                    "workflowVersionId": "workflow-v1",
                    "executionScopeId": "cloud",
                    "algorithm": "heft",
                    "status": "completed",
                    "observedMakespanSeconds": 500,
                    "observedCost": 10,
                },
            ]
        }
        thresholds = derive_thresholds(results, "hybrid", [1.2], "cloud")
        self.assertEqual(thresholds[0]["deadlineSeconds"], 120)
        self.assertEqual(thresholds[0]["budget"], 12)
        self.assertEqual(thresholds[0]["referenceScopeId"], "hybrid")
        self.assertEqual(thresholds[0]["costReferenceScopeId"], "cloud")


class SLACampaignTests(unittest.TestCase):
    def test_builds_one_session_per_threshold_and_environment(self):
        baseline = {
            "records": [
                {
                    "session": {
                        "session": {
                            "workflowVersionId": "workflow-v1",
                            "executionScopeId": "scheduler-a-scope-v1",
                            "networkTopologyId": "scheduler-a-network-v1",
                        }
                    }
                },
                {
                    "session": {
                        "session": {
                            "workflowVersionId": "workflow-v1",
                            "executionScopeId": "scheduler-b-scope-v1",
                            "networkTopologyId": "scheduler-b-network-v1",
                        }
                    }
                },
            ]
        }
        thresholds = {
            "thresholds": [
                {
                    "workflowVersionId": "workflow-v1",
                    "factor": 1.2,
                    "deadlineSeconds": 120,
                    "budget": 12,
                    "referenceScopeId": "scheduler-a-scope-v1",
                    "executionRunId": "heft-reference",
                    "referenceMakespanSeconds": 100,
                    "referenceCost": 10,
                }
            ]
        }
        definitions = build_session_definitions("sla-campaign", baseline, thresholds)
        self.assertEqual(len(definitions), 2)
        self.assertEqual(definitions[0]["deadlineSeconds"], 120)
        self.assertEqual(definitions[1]["budget"], 12)
        self.assertEqual(
            {definition["executionScopeId"] for definition in definitions},
            {"scheduler-a-scope-v1", "scheduler-b-scope-v1"},
        )


class InterferenceCampaignTests(unittest.TestCase):
    def test_builds_seeded_coverage_matrix_for_three_algorithms(self):
        workflow = {
            "id": "workflow-v1",
            "activities": [
                {"id": f"activity-{index}", "activityTypeId": "family"}
                for index in range(10)
            ],
        }
        threshold = {
            "workflowVersionId": "workflow-v1",
            "factor": 1.2,
            "deadlineSeconds": 120,
            "budget": 12,
            "executionRunId": "reference",
        }
        sessions = build_interference_sessions(
            "interference",
            workflow,
            threshold,
            "hybrid-scope",
            "hybrid-topology",
            1.5,
            (0, 20, 100),
            (7, 11),
        )
        self.assertEqual(len(sessions), 6)
        self.assertEqual(sum(len(session["algorithms"]) for session in sessions), 18)
        twenty = next(
            session
            for session in sessions
            if session["configuration"]["selectionSeed"] == 7
            and session["configuration"]["coveragePercent"] == 20
        )
        group = twenty["configuration"]["interferenceMatrix"]["groups"][0]
        self.assertEqual(len(group["activityIds"]), 2)
        self.assertEqual(group["slowdownFactor"], 1.5)
        self.assertEqual(
            [algorithm["id"] for algorithm in twenty["algorithms"]],
            ["prism-time", "prism-cost", "heft"],
        )
        zero = next(
            session for session in sessions if session["configuration"]["coveragePercent"] == 0
        )
        self.assertEqual(zero["configuration"]["interferenceMatrix"]["groups"], [])

    def test_coverage_prefixes_are_nested_for_same_seed(self):
        activity_ids = [f"activity-{index}" for index in range(100)]
        ten = set(selected_activity_ids("workflow", activity_ids, 42, 10))
        twenty = set(selected_activity_ids("workflow", activity_ids, 42, 20))
        fifty = set(selected_activity_ids("workflow", activity_ids, 42, 50))
        self.assertLess(ten, twenty)
        self.assertLess(twenty, fifty)
        self.assertNotEqual(
            ten,
            set(selected_activity_ids("workflow", activity_ids, 43, 10)),
        )


class SearchCampaignTests(unittest.TestCase):
    def setUp(self):
        self.thresholds = {
            "thresholds": [
                {
                    "workflowVersionId": workflow,
                    "factor": 1.2,
                    "deadlineSeconds": 120,
                    "budget": 12,
                    "executionRunId": f"reference-{workflow}",
                }
                for workflow in ("montage-58-v1", "montage-6448-v1")
            ]
        }

    def test_beam_campaign_varies_only_beam_across_montage_sizes(self):
        sessions = build_beam_sessions(
            "beam",
            self.thresholds,
            beams=(1, 20, 120),
        )
        self.assertEqual(len(sessions), 6)
        self.assertEqual(sum(len(session["algorithms"]) for session in sessions), 12)
        self.assertEqual(
            {session["configuration"]["beamWidth"] for session in sessions},
            {1, 20, 120},
        )
        self.assertEqual(
            {algorithm["id"] for session in sessions for algorithm in session["algorithms"]},
            {"prism-time", "prism-cost"},
        )

    def test_reference_campaign_uses_high_search_budget(self):
        sessions = build_reference_sessions("reference", self.thresholds)
        self.assertEqual(len(sessions), 2)
        for session in sessions:
            self.assertEqual(session["configuration"]["beamWidth"], 120)
            self.assertEqual(session["configuration"]["optionCount"], 250)
            self.assertEqual(session["configuration"]["readyBranchLimit"], 16)


if __name__ == "__main__":
    unittest.main()
