package main

import (
	"os"
	"strings"
	"testing"
)

func TestMainDispatchesCLICommand(t *testing.T) {
	previous := os.Args
	t.Cleanup(func() { os.Args = previous })
	os.Args = []string{"akoflow", "unknown"}
	defer func() {
		value := recover()
		message, ok := value.(string)
		if !ok || !strings.Contains(message, "Invalid command") {
			t.Fatalf("panic=%v", value)
		}
	}()
	main()
}
