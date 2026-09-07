package eventloop

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

var ansibleTaskLine = regexp.MustCompile(`^TASK \[(.+)]`)

// ParseCloudOperationLog creates stable, line-addressed events from the raw
// append-only log. Re-parsing while an operation is active is idempotent
// because sequence is the physical line number and the repository ignores a
// duplicate (operation, sequence).
func ParseCloudOperationLog(operationID string, raw []byte) []domain.CloudOperationEvent {
	result := make([]domain.CloudOperationEvent, 0)
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	sequence := 0
	tool, phase, task := "", "", ""
	for scanner.Scan() {
		sequence++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		event := domain.CloudOperationEvent{OperationID: operationID, Sequence: sequence, Timestamp: time.Now().UTC(), Level: "info", Event: "log", Raw: line, Message: line}
		switch {
		case strings.HasPrefix(line, "[Terraform]"):
			tool, phase, event.Tool, event.Phase = "terraform", "provisioning", "terraform", "provisioning"
			event.Message = strings.TrimSpace(strings.TrimPrefix(line, "[Terraform]"))
			if strings.Contains(line, " completed") {
				event.Event = "command.completed"
			} else if strings.Contains(line, " failed") {
				event.Event, event.Level = "command.failed", "error"
			} else {
				event.Event = "command.started"
			}
		case strings.HasPrefix(line, "[Ansible] waiting for SSH"):
			tool, phase, event.Tool, event.Phase, event.Event = "ansible", "waiting-for-ssh", "ansible", "waiting-for-ssh", "ssh.waiting"
		case strings.HasPrefix(line, "[Ansible] SSH ready"):
			tool, phase, event.Tool, event.Phase, event.Event = "ansible", "waiting-for-ssh", "ansible", "waiting-for-ssh", "ssh.ready"
		case strings.HasPrefix(line, "[Ansible]"):
			tool, phase, event.Tool, event.Phase = "ansible", "configuration", "ansible", "configuration"
			event.Message = strings.TrimSpace(strings.TrimPrefix(line, "[Ansible]"))
			if strings.Contains(line, " failed") {
				event.Event, event.Level = "configuration.failed", "error"
			} else if strings.Contains(line, "validated") || strings.Contains(line, " passed") {
				event.Event = "validation.completed"
			}
		case ansibleTaskLine.MatchString(line):
			tool, phase, event.Tool, event.Phase, event.Event = "ansible", "configuration", "ansible", "configuration", "task.started"
			task = ansibleTaskLine.FindStringSubmatch(line)[1]
			event.Task, event.Message = task, task
		case tool == "ansible" && (strings.HasPrefix(line, "ok:") || strings.HasPrefix(line, "changed:") || strings.HasPrefix(line, "skipping:")):
			event.Tool, event.Phase, event.Event, event.Task = tool, phase, "task.completed", task
			event.Host = bracketHost(line)
		case tool == "ansible" && (strings.HasPrefix(line, "fatal:") || strings.HasPrefix(line, "failed:")):
			event.Tool, event.Phase, event.Event, event.Task, event.Level = tool, phase, "task.failed", task, "error"
			event.Host = bracketHost(line)
		case tool == "terraform" && strings.Contains(line, ": Creating"):
			event.Tool, event.Phase, event.Event = tool, phase, "resource.creating"
		case tool == "terraform" && strings.Contains(line, ": Creation complete"):
			event.Tool, event.Phase, event.Event = tool, phase, "resource.created"
		case tool == "terraform" && strings.HasPrefix(line, "Error:"):
			event.Tool, event.Phase, event.Event, event.Level = tool, phase, "resource.failed", "error"
		default:
			event.Tool, event.Phase = tool, phase
		}
		result = append(result, event)
	}
	return result
}

func bracketHost(line string) string {
	start, end := strings.IndexByte(line, '['), strings.IndexByte(line, ']')
	if start >= 0 && end > start {
		return line[start+1 : end]
	}
	return ""
}
