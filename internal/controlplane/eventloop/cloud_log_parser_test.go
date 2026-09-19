package eventloop

import "testing"

func TestParseCloudOperationLogBuildsStableStructuredEvents(t *testing.T) {
	raw := []byte(`[Terraform] terraform apply -no-color
google_compute_instance.worker: Creating...
google_compute_instance.worker: Creation complete after 12s
[Terraform] completed
[Ansible] waiting for SSH connectivity
[Ansible] SSH ready
TASK [Install Docker] ************************************************
changed: [10.0.0.2]
TASK [Validate worker] ************************************************
fatal: [10.0.0.2]: FAILED! => {"msg":"docker missing"}
[Ansible] failed: exit status 2
`)
	events := ParseCloudOperationLog("operation", raw)
	if len(events) != 11 {
		t.Fatalf("events=%d: %#v", len(events), events)
	}
	if events[0].Sequence != 1 || events[0].Tool != "terraform" || events[1].Event != "resource.creating" || events[2].Event != "resource.created" {
		t.Fatalf("terraform events = %#v", events[:4])
	}
	if events[6].Task != "Install Docker" || events[7].Event != "task.completed" || events[7].Host != "10.0.0.2" {
		t.Fatalf("completed Ansible task = %#v %#v", events[6], events[7])
	}
	if events[9].Event != "task.failed" || events[9].Level != "error" || events[9].Task != "Validate worker" {
		t.Fatalf("failed Ansible task = %#v", events[9])
	}
}

func TestParseCloudOperationLogUsesRecordedTimestampsAndPhaseDurations(t *testing.T) {
	raw := []byte(`[2026-09-17T06:10:40Z] [Terraform] terraform apply -no-color
[2026-09-17T06:12:10Z] [Terraform] completed in 90s
[2026-09-17T06:12:11Z] [Ansible] waiting for SSH connectivity
[2026-09-17T06:12:26Z] [Ansible] SSH ready in 15s
[2026-09-17T06:12:27Z] [Ansible] applying playbook.yaml
[2026-09-17T06:13:07Z] [Ansible] playbook completed in 40s; running validation checks
`)
	events := ParseCloudOperationLog("operation", raw)
	if events[1].DurationSeconds != 90 || events[3].DurationSeconds != 15 || events[5].DurationSeconds != 40 {
		t.Fatalf("durations=%v,%v,%v", events[1].DurationSeconds, events[3].DurationSeconds, events[5].DurationSeconds)
	}
	if events[0].Timestamp.Format("15:04:05") != "06:10:40" {
		t.Fatalf("timestamp=%v", events[0].Timestamp)
	}
}
