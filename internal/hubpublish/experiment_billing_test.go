package hubpublish

import (
	"reflect"
	"testing"
)

func TestExecutionCaptureOmitsVolatileBillingQueryTime(t *testing.T) {
	billing := map[string]any{
		"asOf": "2026-01-01T00:00:00Z", "knownCost": 2.5,
		"instances": []any{map[string]any{"instanceId": "instance-1"}},
	}
	detail := map[string]any{"cloudBilling": billing}
	first := exportExecution(detail, nil)
	if billing["asOf"] == nil {
		t.Fatal("export mutated live billing")
	}
	billing["asOf"] = "2026-01-01T00:00:01Z"
	if second := exportExecution(detail, nil); !reflect.DeepEqual(first, second) {
		t.Fatalf("billing query time changed the export: %v vs %v", first, second)
	}
	billing["knownCost"] = 3.0
	if third := exportExecution(detail, nil); reflect.DeepEqual(first, third) {
		t.Fatal("export ignored an observed billing difference")
	}
}
