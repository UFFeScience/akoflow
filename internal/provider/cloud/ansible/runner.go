package ansible

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	root, err := filepath.Abs(r.Root)
	if err != nil {
		return fmt.Errorf("resolve Ansible workspace: %w", err)
	}
	workspace := filepath.Join(root, spec.InstanceID, "ansible")
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
	logFile, logErr := os.OpenFile(filepath.Join(root, spec.InstanceID, "provision.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if logErr != nil {
		return logErr
	}
	defer logFile.Close()
	_, _ = fmt.Fprintln(logFile, "\n[Ansible] waiting for SSH connectivity")
	_ = logFile.Sync()
	if err := waitForSSH(ctx, spec, keyPath, logFile); err != nil {
		_, _ = fmt.Fprintf(logFile, "[Ansible] SSH failed: %v\n", err)
		_ = logFile.Sync()
		return err
	}
	_, _ = fmt.Fprintln(logFile, "[Ansible] SSH ready")
	_ = logFile.Sync()
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
	_, _ = fmt.Fprintf(logFile, "\n[Ansible] applying %s\n", filepath.Base(playbookPath))
	_ = logFile.Sync()
	var output bytes.Buffer
	command.Stdout = io.MultiWriter(&output, logFile)
	command.Stderr = io.MultiWriter(&output, logFile)
	err = command.Run()
	if err != nil {
		_, _ = fmt.Fprintf(logFile, "[Ansible] failed: %v\n", err)
		_ = logFile.Sync()
		return fmt.Errorf("ansible-playbook: %w: %s", err, strings.TrimSpace(output.String()))
	}
	_, _ = fmt.Fprintln(logFile, "[Ansible] playbook completed; running validation checks")
	_ = logFile.Sync()
	for _, check := range spec.Checks {
		_, _ = fmt.Fprintf(logFile, "[Ansible] validating %s\n", check.Name)
		_ = logFile.Sync()
		if err := runCheck(ctx, spec, keyPath, check.Command); err != nil {
			_, _ = fmt.Fprintf(logFile, "[Ansible] validation %s failed: %v\n", check.Name, err)
			_ = logFile.Sync()
			return fmt.Errorf("validation %q: %w", check.Name, err)
		}
		_, _ = fmt.Fprintf(logFile, "[Ansible] validation %s passed\n", check.Name)
		_ = logFile.Sync()
	}
	_, _ = fmt.Fprintln(logFile, "[Ansible] configuration validated")
	_ = logFile.Sync()
	return nil
}

func waitForSSH(ctx context.Context, spec ports.MachineConfigurationSpec, keyPath string, logFile *os.File) error {
	timeout := time.NewTimer(5 * time.Minute)
	defer timeout.Stop()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for attempt := 1; ; attempt++ {
		if err := runCheck(ctx, spec, keyPath, "true"); err == nil {
			return nil
		} else {
			_, _ = fmt.Fprintf(logFile, "[Ansible] SSH attempt %d not ready: %v\n", attempt, err)
			_ = logFile.Sync()
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
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve SSH private key: %w", err)
	}
	info, err := os.Stat(absolutePath)
	if err != nil || info.IsDir() {
		return "", fmt.Errorf("SSH private key is unavailable")
	}
	return absolutePath, nil
}
