package utils_exec_command

import (
	"os"
	"os/exec"
)

type UtilsExecCommand struct {
}

func New() *UtilsExecCommand {
	return &UtilsExecCommand{}
}

func (u *UtilsExecCommand) RunCommand(command string, args ...string) (error, []byte, error) {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	response := cmd.Run()

	// get output
	out, err := cmd.Output()

	return response, out, err
}
