package terraform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
)

type Runner struct {
	Root   string
	Binary string
}

var safeID = regexp.MustCompile(`[^a-z0-9-]+`)

func (r Runner) Apply(ctx context.Context, spec ports.TerraformProvisionSpec) (ports.TerraformResult, error) {
	if spec.Target.Provider != "gcp" {
		return ports.TerraformResult{}, fmt.Errorf("Terraform provider %q is not implemented", spec.Target.Provider)
	}
	workspace, err := r.prepare(spec)
	if err != nil {
		return ports.TerraformResult{}, err
	}
	if _, err = r.run(ctx, workspace, "fmt", "-no-color"); err != nil {
		return ports.TerraformResult{}, err
	}
	if _, err = r.run(ctx, workspace, "init", "-no-color", "-input=false"); err != nil {
		return ports.TerraformResult{}, err
	}
	if _, err = r.run(ctx, workspace, "apply", "-auto-approve", "-no-color", "-input=false", "-var-file=terraform.tfvars.json"); err != nil {
		return ports.TerraformResult{}, err
	}
	output, err := r.run(ctx, workspace, "output", "-json")
	if err != nil {
		return ports.TerraformResult{}, err
	}
	return parseOutput(output)
}

func (r Runner) Destroy(ctx context.Context, instanceID string) error {
	workspace, err := r.workspace(instanceID)
	if err != nil {
		return err
	}
	_, err = r.run(ctx, workspace, "destroy", "-auto-approve", "-no-color", "-input=false", "-var-file=terraform.tfvars.json")
	return err
}

func (r Runner) Stop(ctx context.Context, instanceID string) error {
	_, err := r.setDesiredStatus(ctx, instanceID, "TERMINATED")
	return err
}

func (r Runner) Start(ctx context.Context, instanceID string) (ports.TerraformResult, error) {
	return r.setDesiredStatus(ctx, instanceID, "RUNNING")
}

func (r Runner) setDesiredStatus(ctx context.Context, instanceID, status string) (ports.TerraformResult, error) {
	workspace, err := r.workspace(instanceID)
	if err != nil {
		return ports.TerraformResult{}, err
	}
	variablesPath := filepath.Join(workspace, "terraform.tfvars.json")
	encoded, err := os.ReadFile(variablesPath)
	if err != nil {
		return ports.TerraformResult{}, err
	}
	var values map[string]any
	if err := json.Unmarshal(encoded, &values); err != nil {
		return ports.TerraformResult{}, err
	}
	values["desired_status"] = status
	encoded, _ = json.MarshalIndent(values, "", "  ")
	if err := os.WriteFile(variablesPath, encoded, 0600); err != nil {
		return ports.TerraformResult{}, err
	}
	if _, err = r.run(
		ctx, workspace, "apply", "-auto-approve", "-no-color", "-input=false",
		"-var-file=terraform.tfvars.json",
	); err != nil {
		return ports.TerraformResult{}, err
	}
	output, err := r.run(ctx, workspace, "output", "-json")
	if err != nil {
		return ports.TerraformResult{}, err
	}
	return parseOutput(output)
}

func (r Runner) prepare(spec ports.TerraformProvisionSpec) (string, error) {
	workspace, err := r.workspace(spec.InstanceID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(workspace, 0700); err != nil {
		return "", err
	}
	credentialPath := filepath.Join(workspace, "credentials.json")
	if err := os.WriteFile(credentialPath, spec.Credential, 0600); err != nil {
		return "", fmt.Errorf("write Terraform credential: %w", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "main.tf"), []byte(gcpModule), 0600); err != nil {
		return "", err
	}
	values := map[string]any{
		"credentials_file": credentialPath,
		"project":          stringValue(spec.Target.Configuration, "projectId"),
		"region":           spec.Target.Region,
		"zone":             spec.Target.FixedZone,
		"name":             providerName(spec.InstanceID),
		"machine_type":     spec.Target.ProviderMachineType,
		"image":            spec.Target.ImageReference,
		"disk_type":        stringValue(spec.Target.Configuration, "diskType"),
		"disk_size_gib":    intValue(spec.Target.Configuration, "diskSizeGiB", 30),
		"spot":             spec.Target.ProvisioningMode == "spot",
		"ssh_user":         spec.SSHUser,
		"ssh_public_key":   spec.PublicKey,
		"desired_status":   "RUNNING",
		"network":          stringValue(spec.Target.Configuration, "network"),
		"subnetwork":       stringValue(spec.Target.Configuration, "subnetwork"),
		"ssh_source_ranges": stringSliceValue(
			spec.Target.Configuration, "sshSourceRanges", []string{"0.0.0.0/0"},
		),
	}
	diskAdjusted := strings.HasPrefix(spec.Target.ProviderMachineType, "e2-") &&
		strings.HasPrefix(stringAny(values["disk_type"]), "hyperdisk-")
	if diskAdjusted {
		values["disk_type"] = "pd-balanced"
	}
	if values["disk_type"] == "" {
		values["disk_type"] = "pd-balanced"
	}
	if values["network"] == "" {
		values["network"] = "default"
	}
	encoded, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(workspace, "terraform.tfvars.json"), encoded, 0600); err != nil {
		return "", err
	}
	if diskAdjusted {
		logFile, logErr := os.OpenFile(filepath.Join(workspace, "provision.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if logErr != nil {
			return "", logErr
		}
		_, _ = fmt.Fprintf(logFile, "[Terraform] compatibility: %s does not support %s; using pd-balanced\n", spec.Target.ProviderMachineType, stringValue(spec.Target.Configuration, "diskType"))
		_ = logFile.Sync()
		_ = logFile.Close()
	}
	return workspace, nil
}

func (r Runner) workspace(instanceID string) (string, error) {
	if strings.TrimSpace(r.Root) == "" || !strings.HasPrefix(instanceID, "cloud-instance-") {
		return "", fmt.Errorf("invalid Terraform workspace or instance id")
	}
	root, err := filepath.Abs(r.Root)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, instanceID), nil
}

func (r Runner) run(ctx context.Context, workspace string, arguments ...string) ([]byte, error) {
	binary := strings.TrimSpace(r.Binary)
	if binary == "" {
		binary = "terraform"
	}
	command := exec.CommandContext(ctx, binary, arguments...)
	command.Dir = workspace
	command.Env = append(os.Environ(), "TF_IN_AUTOMATION=1")
	logFile, logErr := os.OpenFile(filepath.Join(workspace, "provision.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if logErr != nil {
		return nil, logErr
	}
	defer logFile.Close()
	_, _ = fmt.Fprintf(logFile, "\n[Terraform] terraform %s\n", strings.Join(arguments, " "))
	_ = logFile.Sync()
	var output bytes.Buffer
	command.Stdout = io.MultiWriter(&output, logFile)
	command.Stderr = io.MultiWriter(&output, logFile)
	err := command.Run()
	if err != nil {
		_, _ = fmt.Fprintf(logFile, "[Terraform] failed: %v\n", err)
		_ = logFile.Sync()
		return nil, fmt.Errorf("terraform %s: %w: %s", arguments[0], err, strings.TrimSpace(output.String()))
	}
	_, _ = fmt.Fprintln(logFile, "[Terraform] completed")
	_ = logFile.Sync()
	return output.Bytes(), nil
}

func (r Runner) Log(_ context.Context, instanceID string) ([]byte, error) {
	workspace, err := r.workspace(instanceID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(workspace, "provision.log"))
	if os.IsNotExist(err) {
		return []byte("Provisioning has started; waiting for Terraform output...\n"), nil
	}
	return data, err
}

func parseOutput(encoded []byte) (ports.TerraformResult, error) {
	var values map[string]struct {
		Value any `json:"value"`
	}
	if err := json.Unmarshal(encoded, &values); err != nil {
		return ports.TerraformResult{}, fmt.Errorf("decode Terraform output: %w", err)
	}
	flat := make(map[string]any, len(values))
	for key, value := range values {
		flat[key] = value.Value
	}
	return ports.TerraformResult{
		ProviderID: stringAny(flat["instance_id"]), PublicAddress: stringAny(flat["public_ip"]),
		PrivateAddress: stringAny(flat["private_ip"]),
		Disk:           map[string]any{"name": flat["disk_name"], "sizeGiB": flat["disk_size_gib"]},
		Output:         flat,
	}, nil
}

func providerName(value string) string {
	value = safeID.ReplaceAllString(strings.ToLower(value), "-")
	value = strings.Trim(value, "-")
	if len(value) > 61 {
		value = value[:61]
	}
	return value
}

func stringValue(values map[string]any, key string) string { return stringAny(values[key]) }

func stringAny(value any) string {
	text, _ := value.(string)
	return text
}

func intValue(values map[string]any, key string, fallback int) int {
	switch value := values[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return fallback
	}
}

func stringSliceValue(values map[string]any, key string, fallback []string) []string {
	value, exists := values[key]
	if !exists {
		return fallback
	}
	switch items := value.(type) {
	case []string:
		return items
	case []any:
		result := make([]string, 0, len(items))
		for _, item := range items {
			if text := strings.TrimSpace(stringAny(item)); text != "" {
				result = append(result, text)
			}
		}
		if len(result) > 0 {
			return result
		}
	case string:
		result := []string{}
		for _, item := range strings.Split(items, ",") {
			if text := strings.TrimSpace(item); text != "" {
				result = append(result, text)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return fallback
}

const gcpModule = `terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}
variable "credentials_file" {
  type      = string
  sensitive = true
}
variable "project" { type = string }
variable "region" { type = string }
variable "zone" {
  type    = string
  default = ""
}
variable "name" { type = string }
variable "machine_type" { type = string }
variable "image" { type = string }
variable "disk_type" { type = string }
variable "disk_size_gib" { type = number }
variable "spot" { type = bool }
variable "ssh_user" { type = string }
variable "ssh_public_key" { type = string }
variable "desired_status" {
  type    = string
  default = "RUNNING"
}
variable "network" { type = string }
variable "subnetwork" {
  type    = string
  default = ""
}
variable "ssh_source_ranges" { type = list(string) }
provider "google" {
  credentials = file(var.credentials_file)
  project     = var.project
  region      = var.region
}
data "google_compute_zones" "available" {
  project = var.project
  region  = var.region
  status  = "UP"
}
locals {
  selected_zone = var.zone != "" ? var.zone : data.google_compute_zones.available.names[0]
}
resource "google_compute_instance" "worker" {
  name                      = var.name
  project                   = var.project
  zone                      = local.selected_zone
  machine_type              = var.machine_type
  desired_status            = var.desired_status
  allow_stopping_for_update = true
  tags                      = [var.name]
  boot_disk {
    initialize_params {
      image = var.image
      size  = var.disk_size_gib
      type  = var.disk_type
    }
  }
  network_interface {
    network    = var.subnetwork == "" ? var.network : null
    subnetwork = var.subnetwork == "" ? null : var.subnetwork
    access_config {}
  }
  metadata = { ssh-keys = "${var.ssh_user}:${var.ssh_public_key}" }
  scheduling {
    provisioning_model = var.spot ? "SPOT" : "STANDARD"
    preemptible         = var.spot
    automatic_restart   = var.spot ? false : true
  }
  labels = { managed-by = "akoflow" }
}
resource "google_compute_firewall" "ssh" {
  name          = "${var.name}-ssh"
  project       = var.project
  network       = var.network
  direction     = "INGRESS"
  source_ranges = var.ssh_source_ranges
  target_tags   = [var.name]
  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
}
output "instance_id" { value = google_compute_instance.worker.instance_id }
output "public_ip" { value = google_compute_instance.worker.network_interface[0].access_config[0].nat_ip }
output "private_ip" { value = google_compute_instance.worker.network_interface[0].network_ip }
output "zone" { value = google_compute_instance.worker.zone }
output "disk_name" { value = google_compute_instance.worker.boot_disk[0].source }
output "disk_size_gib" { value = var.disk_size_gib }
`
