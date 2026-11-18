package local

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type nativeLaunchGate struct {
	reader, writer *os.File
}

func (a *Adapter) nativeCommand(execution domain.ActivityExecutionContext) (*exec.Cmd, *nativeLaunchGate, error) {
	spec := execution.Activity.Command
	if runtime.GOOS != "linux" || !execution.SharedLocalReservation {
		return exec.Command(spec.Entrypoint, spec.Arguments...), nil, nil
	}
	if a.nativeLaunch == nil {
		return nil, nil, fmt.Errorf("native launch recorder is unavailable")
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	args := []string{"-c", `IFS= read -r permission <&3 && [ "$permission" = start ] || exit 125; exec 3<&-; exec "$@"`, "akoflow-native", spec.Entrypoint}
	command := exec.Command("/bin/sh", append(args, spec.Arguments...)...)
	command.ExtraFiles = []*os.File{reader}
	return command, &nativeLaunchGate{reader, writer}, nil
}

func (a *Adapter) startNativeCommand(ctx context.Context, execution domain.ActivityExecutionContext, command *exec.Cmd, gate *nativeLaunchGate) error {
	if gate == nil {
		return command.Start()
	}
	defer gate.reader.Close()
	defer gate.writer.Close()
	if err := command.Start(); err != nil {
		return err
	}
	pid := command.Process.Pid
	identity, _, err := nativeProcessIdentity(pid)
	if err == nil {
		err = a.nativeLaunch.RecordNativeLaunch(ctx, execution.Run.ID, execution.Activity.ID,
			max(1, execution.Attempt), strconv.Itoa(pid), identity)
	}
	if err == nil {
		_, err = gate.writer.WriteString("start\n")
	}
	if err != nil {
		_ = gate.writer.Close()
		_ = command.Wait()
	}
	return err
}
