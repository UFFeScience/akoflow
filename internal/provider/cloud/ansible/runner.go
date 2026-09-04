package ansible

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
)

type Runner struct {
	Root   string
	Binary string
}

func (r Runner) Configure(ctx context.Context, spec ports.MachineConfigurationSpec) error {
	if strings.TrimSpace(spec.Address) == "" {
		return fmt.Errorf("machine configuration requires a public address")
	}
	keyPath, err := privateKeyPath(spec.CredentialRef)
	if err != nil {
		return err
	}
	workspace := filepath.Join(r.Root, spec.InstanceID, "ansible")
	if err := os.MkdirAll(workspace, 0700); err != nil {
		return err
	}
	playbookPath := filepath.Join(workspace, "playbook.yaml")
	variablesPath := filepath.Join(workspace, "variables.json")
	if err := os.WriteFile(playbookPath, []byte(spec.PlaybookYAML), 0600); err != nil {
		return err
	}
	variables, err := json.Marshal(spec.Variables)
	if err != nil {
		return err
	}
	if err := os.WriteFile(variablesPath, variables, 0600); err != nil {
		return err
	}
	if err := waitForSSH(ctx, spec, keyPath); err != nil {
		return err
	}
	binary := r.Binary
	if strings.TrimSpace(binary) == "" {
		binary = "ansible-playbook"
	}
	inventory := spec.Address + ","
	command := exec.CommandContext(
		ctx, binary, "-i", inventory, "-u", spec.SSHUser,
		"--private-key", keyPath, "--extra-vars", "@"+variablesPath, playbookPath,
	)
	command.Dir = workspace
	command.Env = append(os.Environ(), "ANSIBLE_HOST_KEY_CHECKING=False")
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ansible-playbook: %w: %s", err, strings.TrimSpace(string(output)))
	}
	for _, check := range spec.Checks {
		if err := runCheck(ctx, spec, keyPath, check.Command); err != nil {
			return fmt.Errorf("validation %q: %w", check.Name, err)
		}
	}
	return nil
}

func waitForSSH(ctx context.Context, spec ports.MachineConfigurationSpec, keyPath string) error {
	timeout := time.NewTimer(5 * time.Minute)
	defer timeout.Stop()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if runCheck(ctx, spec, keyPath, "true") == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return fmt.Errorf("SSH did not become ready within 5 minutes")
		case <-ticker.C:
		}
	}
}

func runCheck(ctx context.Context, spec ports.MachineConfigurationSpec, keyPath, command string) error {
	arguments := []string{
		"-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null", "-o", "ConnectTimeout=10",
		"-i", keyPath, spec.SSHUser + "@" + spec.Address, command,
	}
	output, err := exec.CommandContext(ctx, "ssh", arguments...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ssh: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func privateKeyPath(reference string) (string, error) {
	if !strings.HasPrefix(reference, "file:") {
		return "", fmt.Errorf("unsupported SSH credential reference")
	}
	path := strings.TrimPrefix(reference, "file:")
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", fmt.Errorf("SSH private key is unavailable")
	}
	return path, nil
}
