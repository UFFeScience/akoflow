package montagetools

import (
	"fmt"
	"testing"
)

func roundFixture() (Object, Object) {
	activities, assignments, tasks := []any{}, []any{}, []any{}
	for i := 0; i < 58; i++ {
		id := fmt.Sprintf("activity-%d", i)
		activities = append(activities, Object{"id": id})
		assignments = append(assignments, Object{"activityId": id, "resourceId": "micro", "planId": "old", "id": id})
		tasks = append(tasks, Object{"activityId": id, "allocatedResourceId": "micro", "status": "completed"})
	}
	return Object{"Workflow": Object{"activities": activities}, "RuntimeAllocations": []any{}, "Plan": Object{"id": "old", "assignments": assignments}, "Run": Object{"id": "old"}}, Object{"tasks": tasks}
}

func TestInstrumentedRoundPreservesForecastAndStartsFresh(t *testing.T) {
	request, simulation := roundFixture()
	prepared := prepareInstrumentedRound(request, simulation, "montage-fresh", "montage-plan")
	if obj(request["Run"])["id"] != "old" || obj(prepared["Run"])["mode"] != "real" || obj(prepared["Plan"])["id"] != "montage-plan" {
		t.Fatal("source mutated or identity not renewed")
	}
	for _, assignment := range rows(obj(prepared["Plan"])["assignments"]) {
		if assignment["planId"] != "montage-plan" || assignment["resourceId"] != "micro" {
			t.Fatal("placement changed or stale plan relation")
		}
	}
	if obj(obj(prepared["Plan"])["metadata"])["dominantTransferGateAccepted"] != false {
		t.Fatal("preparation cannot accept the scientific gate")
	}
}

func TestInstrumentedRoundRejectsReusedOrMismatchedForecast(t *testing.T) {
	for _, mode := range []string{"allocation", "placement", "failed", "duplicate"} {
		t.Run(mode, func(t *testing.T) {
			request, simulation := roundFixture()
			switch mode {
			case "allocation":
				request["RuntimeAllocations"] = []any{Object{"instanceId": "old"}}
			case "placement":
				obj(arr(simulation["tasks"])[0])["allocatedResourceId"] = "other"
			case "failed":
				obj(arr(simulation["tasks"])[0])["status"] = "failed"
			case "duplicate":
				obj(arr(simulation["tasks"])[1])["activityId"] = "activity-0"
			}
			defer func() {
				if recover() == nil {
					t.Fatal("invalid fresh round accepted")
				}
			}()
			prepareInstrumentedRound(request, simulation, "montage-fresh", "montage-plan")
		})
	}
}
