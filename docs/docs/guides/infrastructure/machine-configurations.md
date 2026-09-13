---
title: Prepare a cloud worker with Ansible
description: Validate and version an Ansible playbook for a cloud capacity target.
---

# Prepare a cloud worker with Ansible

Use a machine configuration when a provisioned Google Cloud worker needs packages or setup before a run. Create and validate a playbook, save a version, then attach that version to a [cloud capacity target](/docs/guides/infrastructure/cloud-capacity#create-a-capacity-target). A saved playbook does not provision a VM.

For the API commands below, complete [API connection setup](/docs/tutorials/api-access) first.

## Using AkôFlow Desktop

Open **Infrastructure → Machine configurations**. Create a named configuration, edit its Ansible playbook, validate it, and save a version. Existing capacity targets refer to a specific configuration-version ID, not to mutable editor contents.

## Using the API

Validate YAML before saving it:

```bash
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/json' -X POST \
  "$AKOFLOW_API_URL/machine-configuration-validations/" \
  -d '{"playbookYaml":"---\n- name: Configure worker\n  hosts: all\n  become: true\n  tasks:\n    - name: Install curl\n      ansible.builtin.package:\n        name: curl\n        state: present\n"}'
```

Create the configuration and then its first version:

```bash
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/json' -X POST "$AKOFLOW_API_URL/machine-configurations/" \
  -d '{"id":"analysis-worker","name":"Analysis worker","description":"Packages used by analysis jobs"}'

curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/json' -X POST \
  "$AKOFLOW_API_URL/machine-configurations/analysis-worker/versions/" \
  -d '{"version":1,"status":"published","playbookYaml":"---\n- name: Configure worker\n  hosts: all\n  become: true\n  tasks:\n    - name: Install curl\n      ansible.builtin.package:\n        name: curl\n        state: present\n","compatibility":{"providers":["gcp"]}}' \
  -o machine-configuration.json || exit 1

AKOFLOW_MACHINE_CONFIGURATION_VERSION_ID=$(jq -er '.versions[] | select(.version == 1) | .id' machine-configuration.json) || exit 1
```

Validation checks playbook structure and returns `valid`, a content hash, and errors when present. It does not provision a machine or execute the playbook. The version request returns the configuration with its versions. Run the [capacity-target example](/docs/guides/infrastructure/cloud-capacity#create-a-capacity-target) in the same Bash session: it attaches this saved version when `AKOFLOW_MACHINE_CONFIGURATION_VERSION_ID` is set. Without that variable, it creates a target using only the built-in worker configuration.
