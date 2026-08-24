package workflow

import "testing"

func TestActivityValidatesExecutionCapabilities(t *testing.T) {
	simulation := &ActivitySimulation{DurationSeconds: 4}
	activity := Activity{ID: "a", Name: "analysis", Kind: ActivityKindTask,
		Capabilities: []ActivityCapability{ActivityCapabilityReal, ActivityCapabilitySimulation},
		Command:      ActivityCommand{Entrypoint: "python"}, Simulation: simulation}
	if err := activity.Validate(); err != nil {
		t.Fatal(err)
	}
	if !activity.Supports(ActivityCapabilityReal) || activity.Supports(ActivityCapabilityInteractive) {
		t.Fatal("unexpected capabilities")
	}
	activity.Command.Entrypoint = ""
	if err := activity.Validate(); err == nil {
		t.Fatal("real execution without entrypoint must fail")
	}
}

func TestInteractiveActivityRequiresServiceDefinition(t *testing.T) {
	activity := Activity{ID: "shell", Name: "shell", Kind: ActivityKindInteractive,
		Capabilities: []ActivityCapability{ActivityCapabilityInteractive}}
	if err := activity.Validate(); err == nil {
		t.Fatal("interactive activity without service definition must fail")
	}
	activity.Service = &ServiceSpec{Ports: []int{22}}
	if err := activity.Validate(); err != nil {
		t.Fatal(err)
	}
}
