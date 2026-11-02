package montagetools

import "testing"

func TestDependencyReferenceExcludesSharedBarrierAndRecoveryReuse(t *testing.T) {
	real := Object{"run": Object{"status": "completed", "id": "real"}, "artifactTransferRuns": []any{
		Object{
			"status":       "completed",
			"startedAt":    1.0,
			"strategy":     "direct-runtime",
			"networkBytes": 100.0,
			"route": Object{
				"producerActivityId": "a",
				"consumerActivityId": "b",
				"phases":             []any{Object{"name": "upload", "durationSeconds": 4.0}, Object{"name": "shared-preparation-barrier", "durationSeconds": 10.0}, Object{"name": "shared-conflict-check-copy-and-verify", "durationSeconds": 3.0}},
			},
		},
		Object{"status": "completed", "startedAt": 2.0, "strategy": "use-existing", "route": Object{"producerActivityId": "c", "consumerActivityId": "b"}},
	}}
	sim := Object{"transfers": []any{Object{"producerActivityId": "a", "consumerActivityId": "b", "strategy": "direct-runtime", "durationSeconds": 2.0, "networkBytes": 100.0}}}
	report := dependencyReference(real, sim)
	rows := rows(report["dependencies"])
	if rows[0]["observedDependencyWallSeconds"] != 4.0 || rows[0]["durationErrorPercent"] != -50.0 || rows[1]["comparable"] != false {
		t.Fatal("shared merge or recovery reuse counted as dependency service")
	}
}
