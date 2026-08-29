package command

import (
	"flag"
	"os"
	"strings"
	"testing"
)

func TestNewReturnsRegisteredCommandAndRejectsUnknownName(t *testing.T) {
	if _, ok := New("run").(*Run); !ok {
		t.Fatalf("registered run command has type %T", New("run"))
	}
	defer func() {
		value := recover()
		if value == nil || !strings.Contains(value.(string), "Invalid command") {
			t.Fatalf("panic=%v", value)
		}
	}()
	New("unknown")
}

func TestRunRejectsInvalidWorkflowFileBeforeConnecting(t *testing.T) {
	previousArgs, previousFlags := os.Args, flag.CommandLine
	t.Cleanup(func() {
		os.Args, flag.CommandLine = previousArgs, previousFlags
	})
	os.Args = []string{"akoflow", "run", "-file", "/missing/workflow.yaml"}
	flag.CommandLine = flag.NewFlagSet("akoflow-test", flag.ContinueOnError)
	defer func() {
		value := recover()
		if value != "Invalid file" {
			t.Fatalf("panic=%v", value)
		}
	}()
	(&Run{}).Run()
}
