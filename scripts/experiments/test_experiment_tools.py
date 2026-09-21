import unittest
from unittest.mock import patch
import gzip
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
from launch_frontier_simulations import bounded_candidates, nondominated_candidates
from analyze_frontiers import analyze, hypervolume, multiplicative_epsilon, pareto_records
from analyze_campaign_results import analyze_campaign, read_json


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

    def test_analysis_reader_accepts_gzip_json(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "results.json.gz"
            expected = {"records": [{"id": "run-1"}]}
            with gzip.open(path, "wt", encoding="utf-8") as output:
                json.dump(expected, output)
            self.assertEqual(read_json(path), expected)


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

    def test_feasible_candidate_is_preferred_over_infeasible_candidate(self):
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

    def test_infeasible_candidate_is_selected_when_it_is_the_only_plan(self):
        candidates = [
            {
                "id": "heft-standard-plan",
                "algorithm": "heft",
                "feasible": False,
                "predicted": {"makespanSeconds": 12, "cost": 3},
            }
        ]

        self.assertEqual(
            choose_candidate(candidates, "heft")["id"],
            "heft-standard-plan",
        )

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


class FrontierTests(unittest.TestCase):
    def candidate(self, identifier, time, cost, algorithm="prism-time"):
        return {
            "id": identifier,
            "fingerprint": identifier,
            "algorithm": algorithm,
            "feasible": True,
            "predicted": {"makespanSeconds": time, "cost": cost},
        }

    def record(self, identifier, time, cost, algorithm="prism-time"):
        return {
            "executionRunId": identifier,
            "workflowVersionId": "workflow",
            "algorithm": algorithm,
            "status": "completed",
            "observedMakespanSeconds": time,
            "observedCost": cost,
        }

    def test_predicted_frontier_removes_dominated_candidates_and_bounds_sample(self):
        candidates = [
            self.candidate("fast", 1, 10),
            self.candidate("middle", 5, 5),
            self.candidate("cheap", 10, 1),
            self.candidate("dominated", 10, 10),
        ]
        self.assertEqual(
            [item["id"] for item in nondominated_candidates(candidates)],
            ["fast", "middle", "cheap"],
        )
        bounded = bounded_candidates(candidates, 2)
        self.assertEqual([item["id"] for item in bounded], ["fast", "cheap"])

    def test_common_simulator_frontier_metrics(self):
        records = [
            self.record("fast", 1, 10, "prism-time"),
            self.record("middle", 5, 5, "prism-time"),
            self.record("cheap", 10, 1, "prism-cost"),
            self.record("dominated", 10, 10, "prism-cost"),
        ]
        frontier = pareto_records(records)
        self.assertEqual([item["executionRunId"] for item in frontier], ["fast", "middle", "cheap"])
        self.assertEqual(hypervolume(frontier, (11, 11)), 44)
        epsilon = multiplicative_epsilon(
            [record for record in frontier if record["algorithm"] == "prism-time"],
            frontier,
        )
        self.assertEqual(epsilon, 5)
        result = analyze(records)
        self.assertEqual(len(result["workflows"]), 1)
        self.assertEqual(len(result["metrics"]), 2)
        prism_time = next(
            row for row in result["metrics"] if row["algorithm"] == "prism-time"
        )
        self.assertEqual(prism_time["bestMakespanGapPercent"], 0)
        self.assertEqual(prism_time["bestCostGapPercent"], 400)
        self.assertEqual(prism_time["meanDistanceToReference"], 0)
        self.assertGreater(prism_time["frontierSpan"], 0)


class CampaignAnalysisTests(unittest.TestCase):
    def test_joins_planning_and_scores_observed_results(self):
        planning = {
            "records": [
                {
                    "session": {
                        "session": {
                            "id": "session",
                            "deadlineSeconds": 12,
                            "budget": 6,
                        },
                        "algorithmRuns": [
                            {
                                "algorithm": "heft",
                                "status": "completed",
                                "candidateCount": 1,
                                "startedAt": "2026-01-01T00:00:00Z",
                                "completedAt": "2026-01-01T00:00:02Z",
                            },
                            {
                                "algorithm": "prism-time",
                                "status": "completed",
                                "candidateCount": 2,
                                "configuration": {"beamWidth": 20},
                                "startedAt": "2026-01-01T00:00:00Z",
                                "completedAt": "2026-01-01T00:00:05Z",
                            },
                        ],
                    }
                }
            ]
        }
        simulations = {
            "records": [
                {
                    "sessionId": "session",
                    "executionRunId": "heft",
                    "workflowVersionId": "workflow",
                    "executionScopeId": "scope",
                    "algorithm": "heft",
                    "status": "completed",
                    "observedMakespanSeconds": 10,
                    "observedCost": 5,
                    "predictionErrorMakespan": 0.1,
                    "predictionErrorCost": 0.1,
                    "candidateFeasible": False,
                    "selectionPolicy": "best-available-infeasible",
                },
                {
                    "sessionId": "session",
                    "executionRunId": "prism-time",
                    "workflowVersionId": "workflow",
                    "executionScopeId": "scope",
                    "algorithm": "prism-time",
                    "status": "completed",
                    "observedMakespanSeconds": 8,
                    "observedCost": 7,
                    "predictionErrorMakespan": 0.2,
                    "predictionErrorCost": 0.2,
                },
            ]
        }
        result = analyze_campaign(planning, simulations)
        self.assertEqual(len(result["runRows"]), 2)
        self.assertTrue(result["runRows"][0]["slaSatisfied"])
        self.assertFalse(result["runRows"][1]["slaSatisfied"])
        self.assertEqual(result["winnerCounts"]["makespan"], {"prism-time": 1.0})
        self.assertEqual(result["winnerCounts"]["cost"], {"heft": 1.0})
        self.assertFalse(result["instanceComparisons"][0]["runs"][0]["candidateFeasible"])

    def test_does_not_mix_sla_variants_of_the_same_workflow_and_scope(self):
        planning = {
            "records": [
                {
                    "session": {
                        "session": {
                            "id": session_id,
                            "deadlineSeconds": deadline,
                            "budget": 10,
                            "configuration": {"slaFactor": factor},
                        },
                        "algorithmRuns": [],
                    }
                }
                for session_id, deadline, factor in (
                    ("session-f110", 11, 1.1),
                    ("session-f120", 12, 1.2),
                )
            ]
        }
        simulations = {
            "records": [
                {
                    "sessionId": session_id,
                    "executionRunId": session_id + "-heft",
                    "workflowVersionId": "workflow",
                    "executionScopeId": "scope",
                    "algorithm": "heft",
                    "status": "completed",
                    "observedMakespanSeconds": observed,
                    "observedCost": 1,
                }
                for session_id, observed in (
                    ("session-f110", 11.5),
                    ("session-f120", 11.5),
                )
            ]
        }

        result = analyze_campaign(planning, simulations)

        self.assertEqual(len(result["instanceComparisons"]), 2)
        self.assertFalse(result["instanceComparisons"][0]["runs"][0]["slaSatisfied"])
        self.assertTrue(result["instanceComparisons"][1]["runs"][0]["slaSatisfied"])

    def test_summarizes_seeded_interference_against_matching_control(self):
        planning = {
            "records": [
                {
                    "session": {
                        "session": {
                            "id": session_id,
                            "deadlineSeconds": 20,
                            "budget": 20,
                            "configuration": {
                                "experiment": "2-seeded-activity-coverage-slowdown",
                                "selectionSeed": seed,
                                "coveragePercent": coverage,
                                "slowdownFactor": 1.5,
                            },
                        },
                        "algorithmRuns": [
                            {
                                "algorithm": "prism-time",
                                "status": "completed",
                                "startedAt": "2026-01-01T00:00:00Z",
                                "completedAt": "2026-01-01T00:00:02Z",
                            }
                        ],
                    }
                }
                for session_id, seed, coverage in (
                    ("seed-1-control", 1, 0),
                    ("seed-1-half", 1, 50),
                    ("seed-2-control", 2, 0),
                    ("seed-2-half", 2, 50),
                )
            ]
        }
        simulations = {
            "records": [
                {
                    "sessionId": session_id,
                    "executionRunId": session_id + "-prism-time",
                    "workflowVersionId": "workflow",
                    "executionScopeId": "scope",
                    "algorithm": "prism-time",
                    "status": "completed",
                    "observedMakespanSeconds": makespan,
                    "observedCost": cost,
                    "breakdown": {"interferenceSeconds": interference},
                }
                for session_id, makespan, cost, interference in (
                    ("seed-1-control", 10, 5, 0),
                    ("seed-1-half", 15, 6, 3),
                    ("seed-2-control", 8, 4, 0),
                    ("seed-2-half", 10, 5, 2),
                )
            ]
        }

        result = analyze_campaign(planning, simulations)

        half = next(
            row
            for row in result["interferenceSummaries"]
            if row["coveragePercent"] == 50
        )
        self.assertEqual(half["seedCount"], 2)
        self.assertEqual(half["completedCount"], 2)
        self.assertAlmostEqual(half["makespanDegradationPercent"]["mean"], 37.5)
        self.assertAlmostEqual(half["costDegradationPercent"]["mean"], 22.5)
        self.assertEqual(half["observedInterferenceSeconds"]["median"], 2.5)

    def test_selects_smallest_beam_that_meets_quality_rule(self):
        cases = (
            ("small-b1", "small", 1, 12),
            ("small-b4", "small", 4, 10),
            ("large-b1", "large", 1, 23),
            ("large-b4", "large", 4, 20),
        )
        planning = {
            "records": [
                {
                    "session": {
                        "session": {
                            "id": session_id,
                            "configuration": {"experiment": "4-beam-calibration"},
                        },
                        "algorithmRuns": [
                            {
                                "algorithm": "prism-time",
                                "status": "completed",
                                "configuration": {"beamWidth": beam},
                                "startedAt": "2026-01-01T00:00:00Z",
                                "completedAt": "2026-01-01T00:00:02Z",
                            }
                        ],
                    }
                }
                for session_id, _, beam, _ in cases
            ]
        }
        simulations = {
            "records": [
                {
                    "sessionId": session_id,
                    "executionRunId": session_id + "-prism-time",
                    "workflowVersionId": workflow,
                    "executionScopeId": "scope",
                    "algorithm": "prism-time",
                    "status": "completed",
                    "observedMakespanSeconds": makespan,
                    "observedCost": 1,
                }
                for session_id, workflow, _, makespan in cases
            ]
        }

        result = analyze_campaign(planning, simulations)

        self.assertEqual(result["selectedBeams"], {"prism-time": 4})
        beam_one = next(
            row for row in result["beamSummaries"] if row["beamWidth"] == 1
        )
        beam_four = next(
            row for row in result["beamSummaries"] if row["beamWidth"] == 4
        )
        self.assertFalse(beam_one["qualifiesByRule"])
        self.assertTrue(beam_four["selectedByRule"])
        small_four = next(
            row
            for row in result["beamComparisons"]
            if row["workflowVersionId"] == "small" and row["beamWidth"] == 4
        )
        self.assertAlmostEqual(
            small_four["marginalObjectiveImprovementPercent"], 100 / 6
        )


if __name__ == "__main__":
    unittest.main()
