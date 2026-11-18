package local

import (
	"os/exec"
	"strconv"
	"testing"
)

func TestNativeReservationTracksExactProcess(t *testing.T) {
	command := exec.Command("sleep", "30")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	})
	pid := command.Process.Pid
	identity, _, err := nativeProcessIdentity(pid)
	if err != nil {
		t.Skipf("process identity unavailable: %v", err)
	}
	argument := strconv.Itoa(pid)
	if NativeReservationReleased(argument, identity) || NativeReservationReleased(argument, "") {
		t.Fatal("live process or unknown identity released")
	}
	if !NativeReservationReleased(argument, identity+"-different") {
		t.Fatal("different process identity held the reservation")
	}
	if err := command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = command.Wait()
	if !NativeReservationReleased(argument, identity) {
		t.Fatal("terminated process retained the reservation")
	}
}
