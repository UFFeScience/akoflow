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
var timestampedLogLine = regexp.MustCompile(`^\[([^]]+)]\s+(.*)$`)

// ParseCloudOperationLog creates stable, line-addressed events from the raw
// append-only log. Re-parsing while an operation is active is idempotent
// because sequence is the physical line number and the repository ignores a
// duplicate (operation, sequence).
func ParseCloudOperationLog(operationID string, raw []byte) []domain.CloudOperationEvent {
	result := make([]domain.CloudOperationEvent, 0)
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	sequence := 0
	tool, phase, task := "", "", ""
	var terraformStartedAt, sshStartedAt, configurationStartedAt time.Time
	for scanner.Scan() {
		sequence++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		timestamp, content, timestamped := parseLogTimestamp(line)
		event := domain.CloudOperationEvent{OperationID: operationID, Sequence: sequence, Timestamp: timestamp, Level: "info", Event: "log", Raw: line, Message: content}
		line = content
		switch {
		case strings.HasPrefix(line, "[Terraform]"):
			tool, phase, event.Tool, event.Phase = "terraform", "provisioning", "terraform", "provisioning"
			event.Message = strings.TrimSpace(strings.TrimPrefix(line, "[Terraform]"))
			if strings.Contains(line, " completed") {
				event.Event = "command.completed"
				event.DurationSeconds = observedDuration(terraformStartedAt, timestamp, timestamped)
			} else if strings.Contains(line, " failed") {
				event.Event, event.Level = "command.failed", "error"
				event.DurationSeconds = observedDuration(terraformStartedAt, timestamp, timestamped)
			} else {
				event.Event = "command.started"
				if timestamped {
					terraformStartedAt = timestamp
				}
			}
		case strings.HasPrefix(line, "[Ansible] waiting for SSH"):
			tool, phase, event.Tool, event.Phase, event.Event = "ansible", "waiting-for-ssh", "ansible", "waiting-for-ssh", "ssh.waiting"
			if timestamped {
				sshStartedAt = timestamp
			}
		case strings.HasPrefix(line, "[Ansible] SSH ready"):
			tool, phase, event.Tool, event.Phase, event.Event = "ansible", "waiting-for-ssh", "ansible", "waiting-for-ssh", "ssh.ready"
			event.DurationSeconds = observedDuration(sshStartedAt, timestamp, timestamped)
		case strings.HasPrefix(line, "[Ansible]"):
			tool, phase, event.Tool, event.Phase = "ansible", "configuration", "ansible", "configuration"
			event.Message = strings.TrimSpace(strings.TrimPrefix(line, "[Ansible]"))
			if strings.Contains(line, " failed") {
				event.Event, event.Level = "configuration.failed", "error"
			} else if strings.Contains(line, "validated") || strings.Contains(line, " passed") {
				event.Event = "validation.completed"
			} else if strings.Contains(line, "applying ") {
				event.Event = "configuration.started"
				if timestamped {
					configurationStartedAt = timestamp
				}
			} else if strings.Contains(line, "playbook completed") {
				event.Event = "configuration.completed"
				event.DurationSeconds = observedDuration(configurationStartedAt, timestamp, timestamped)
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

func observedDuration(startedAt, finishedAt time.Time, timestamped bool) float64 {
	if !timestamped || startedAt.IsZero() {
		return 0
	}
	return finishedAt.Sub(startedAt).Seconds()
}

func parseLogTimestamp(line string) (time.Time, string, bool) {
	match := timestampedLogLine.FindStringSubmatch(line)
	if len(match) == 3 {
		if value, err := time.Parse(time.RFC3339Nano, match[1]); err == nil {
			return value.UTC(), strings.TrimSpace(match[2]), true
		}
	}
	return time.Now().UTC(), line, false
}

func bracketHost(line string) string {
	start, end := strings.IndexByte(line, '['), strings.IndexByte(line, ']')
	if start >= 0 && end > start {
		return line[start+1 : end]
	}
	return ""
}
