package machineconfiguration

import "testing"

func TestValidatePlaybook(t *testing.T) {
	valid := ValidatePlaybook("- hosts: all\n  tasks:\n    - ansible.builtin.debug:\n        msg: ready\n")
	if !valid.Valid || len(valid.SHA256) != 71 {
		t.Fatalf("expected valid content-addressed playbook, got %#v", valid)
	}
	for name, source := range map[string]string{
		"empty":        "",
		"invalid yaml": "- hosts: [",
		"not plays":    "hosts: all",
		"no hosts":     "- tasks:\n    - debug: {}",
		"no tasks":     "- hosts: all",
	} {
		t.Run(name, func(t *testing.T) {
			if result := ValidatePlaybook(source); result.Valid || len(result.Errors) == 0 {
				t.Fatalf("expected invalid playbook, got %#v", result)
			}
		})
	}
}
