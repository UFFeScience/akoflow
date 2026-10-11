// Package materializationmodel describes a frozen causal preparation estimate.
// It does not select live SSH routes or grant shared-storage trust. Those remain
// executor decisions; the model must carry evidence for the matching strategy.
package materializationmodel

import (
	"encoding/json"
	"fmt"
	"math"
)

type Model struct {
	SchemaVersion            int                       `json:"schemaVersion"`
	Source                   string                    `json:"source"`
	RelayResourceID          string                    `json:"relayResourceId"`
	Resources                map[string]Resource       `json:"resources"`
	Profiles                 []Profile                 `json:"profiles"`
	MergeProfiles            []MergeProfile            `json:"mergeProfiles"`
	InputObservationProfiles []InputObservationProfile `json:"inputObservationProfiles,omitempty"`
	PublicationProfiles      []PublicationProfile      `json:"publicationProfiles,omitempty"`
	AdmissionProfiles        []AdmissionProfile        `json:"admissionProfiles,omitempty"`
}

// Admission is external submit-to-job-start wall, not CPU service or merge.
type AdmissionProfile struct {
	TargetClass  string  `json:"targetClass"`
	RuntimeID    string  `json:"runtimeId"`
	Source       string  `json:"source"`
	Samples      int     `json:"samples"`
	FixedSeconds float64 `json:"fixedSeconds"`
}

// Publication follows scientific compute and gates both dependencies and lane
// release. Scan and inherited-input cleanup are service, not compute or network.
type PublicationProfile struct {
	FixedSeconds                float64 `json:"fixedSeconds,omitempty"`
	TargetClass                 string  `json:"targetClass"`
	Source                      string  `json:"source"`
	Samples                     int     `json:"samples"`
	ScanSecondsPerWorkspaceByte float64 `json:"scanSecondsPerWorkspaceByte"`
	CleanupSecondsPerInputByte  float64 `json:"cleanupSecondsPerInputByte"`
}

// Input observation occurs after merge and before the command. Its measured
// service must exclude admission wait and must not already be in merge/compute.
type InputObservationProfile struct {
	FixedSeconds         float64 `json:"fixedSeconds,omitempty"`
	TargetClass          string  `json:"targetClass"`
	Source               string  `json:"source"`
	Samples              int     `json:"samples"`
	SecondsPerUniqueByte float64 `json:"secondsPerUniqueByte"`
}

type Resource struct {
	ExecutionRuntimeID string `json:"executionRuntimeId,omitempty"`
	Class              string `json:"class"`
	Domain             string `json:"domain"` // cloud, hpc or relay
	// An equal nonempty identity is usable only with explicit proof on both.
	SharedStorageID     string `json:"sharedStorageId,omitempty"`
	SharedStorageProven bool   `json:"sharedStorageProven,omitempty"`
	BulkServiceGroupID  string `json:"bulkServiceGroupId,omitempty"`
	BulkServiceSlots    int    `json:"bulkServiceSlots,omitempty"`
	BulkServiceSource   string `json:"bulkServiceSource,omitempty"`
}

type Profile struct {
	Strategy                     string  `json:"strategy"`
	SourceClass                  string  `json:"sourceClass"`
	TargetClass                  string  `json:"targetClass"`
	Source                       string  `json:"source"`
	Samples                      int     `json:"samples"`
	ArchiveToWorkspaceRatio      float64 `json:"archiveToWorkspaceRatio"`
	ProbeSeconds                 float64 `json:"probeSeconds"`
	SnapshotSecondsPerByte       float64 `json:"snapshotSecondsPerByte"`
	IngressArchiveBytesPerSecond float64 `json:"ingressArchiveBytesPerSecond"`
	ValidationSecondsPerByte     float64 `json:"validationSecondsPerByte"`
	UploadArchiveBytesPerSecond  float64 `json:"uploadArchiveBytesPerSecond"`
	ReceiverSecondsPerByte       float64 `json:"receiverSecondsPerByte"`
	// Pack-and-receive is measured as one combined phase in real telemetry.
	// Do not add source CPU preparation a second time to this effective rate.
	IngressIncludesSourcePreparation bool `json:"ingressIncludesSourcePreparation,omitempty"`
	// Independent service measurements must exclude admission queue. Without
	// this evidence, ingress remains a combined pack-and-receive measurement.
	SeparatedIngressServiceSource string `json:"separatedIngressServiceSource,omitempty"`
}

type MergeProfile struct {
	TargetClass                string  `json:"targetClass"`
	Source                     string  `json:"source"`
	Samples                    int     `json:"samples"`
	CommandOverheadSeconds     float64 `json:"commandOverheadSeconds,omitempty"`
	CheckSecondsPerInputByte   float64 `json:"checkSecondsPerInputByte"`
	CopySecondsPerUniqueByte   float64 `json:"copySecondsPerUniqueByte"`
	VerifySecondsPerUniqueByte float64 `json:"verifySecondsPerUniqueByte"`
}

type Stage struct {
	Name                    string  `json:"name"`
	SourceResourceID        string  `json:"sourceResourceId"`
	TargetResourceID        string  `json:"targetResourceId"`
	Bytes                   int64   `json:"bytes"`
	EffectiveBytesPerSecond float64 `json:"effectiveBytesPerSecond,omitempty"`
	BeforeSeconds           float64 `json:"beforeSeconds,omitempty"`
	AfterSeconds            float64 `json:"afterSeconds,omitempty"`
	ServiceGroupID          string  `json:"serviceGroupId,omitempty"`
	ServiceSlots            int     `json:"serviceSlots,omitempty"`
}

type Route struct {
	Strategy          string  `json:"strategy"`
	LogicalBytes      int64   `json:"logicalBytes"`
	NetworkBytes      int64   `json:"networkBytes"`
	RelayNetworkBytes int64   `json:"relayNetworkBytes"`
	Stages            []Stage `json:"stages"`
}

func Decode(metadata map[string]any) (*Model, error) {
	raw := metadata["workspaceMaterializationModel"]
	if raw == nil {
		return nil, nil
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var model Model
	if err = json.Unmarshal(payload, &model); err != nil {
		return nil, fmt.Errorf("decode workspace materialization model: %w", err)
	}
	if err = model.Validate(); err != nil {
		return nil, err
	}
	return &model, nil
}

func validNonnegative(values ...float64) bool {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return false
		}
	}
	return true
}

func (m Model) Validate() error {
	if (m.SchemaVersion != 1 && m.SchemaVersion != 2) || m.Source == "" {
		return fmt.Errorf("materialization model requires supported version and provenance")
	}
	if m.SchemaVersion == 1 {
		if len(m.AdmissionProfiles) > 0 {
			return fmt.Errorf("runtime admission requires materialization schema version 2")
		}
		for _, p := range m.InputObservationProfiles {
			if p.FixedSeconds != 0 {
				return fmt.Errorf("fixed stage service requires materialization schema version 2")
			}
		}
		for _, p := range m.PublicationProfiles {
			if p.FixedSeconds != 0 {
				return fmt.Errorf("fixed stage service requires materialization schema version 2")
			}
		}
	}
	if relay, exists := m.Resources[m.RelayResourceID]; !exists || relay.Domain != "relay" {
		return fmt.Errorf("materialization model requires an explicit relay resource")
	}
	serviceLimits := map[string]int{}
	for id, resource := range m.Resources {
		if id == "" || resource.Class == "" || (resource.Domain != "cloud" && resource.Domain != "hpc" && resource.Domain != "relay") {
			return fmt.Errorf("invalid materialization resource %q", id)
		}
		if resource.SharedStorageProven && resource.SharedStorageID == "" {
			return fmt.Errorf("shared storage proof requires identity for %q", id)
		}
		if resource.BulkServiceGroupID != "" {
			if resource.Domain == "relay" || resource.BulkServiceSlots < 1 || resource.BulkServiceSource == "" {
				return fmt.Errorf("bulk service %q requires positive slots and evidence", id)
			}
			if n, ok := serviceLimits[resource.BulkServiceGroupID]; ok && n != resource.BulkServiceSlots {
				return fmt.Errorf("inconsistent bulk service slots")
			}
			serviceLimits[resource.BulkServiceGroupID] = resource.BulkServiceSlots
		} else if resource.BulkServiceSlots != 0 || resource.BulkServiceSource != "" {
			return fmt.Errorf("bulk service resource %q lacks group identity", id)
		}
	}
	seen := map[string]bool{}
	for _, profile := range m.Profiles {
		key := profile.Strategy + "\x00" + profile.SourceClass + "\x00" + profile.TargetClass
		if seen[key] {
			return fmt.Errorf("duplicate materialization profile %q", key)
		}
		seen[key] = true
		if profile.Source == "" || profile.Samples < 1 || profile.SourceClass == "" || profile.TargetClass == "" || !validNonnegative(profile.ArchiveToWorkspaceRatio, profile.ProbeSeconds, profile.SnapshotSecondsPerByte, profile.IngressArchiveBytesPerSecond, profile.ValidationSecondsPerByte, profile.UploadArchiveBytesPerSecond, profile.ReceiverSecondsPerByte) {
			return fmt.Errorf("invalid materialization profile %q", key)
		}
		if profile.Strategy != "gateway" && profile.Strategy != "direct-runtime" && profile.Strategy != "shared-storage" {
			return fmt.Errorf("unknown materialization strategy %q", profile.Strategy)
		}
		if profile.Strategy != "shared-storage" && (profile.ArchiveToWorkspaceRatio <= 0 || profile.UploadArchiveBytesPerSecond <= 0) {
			return fmt.Errorf("materialization profile lacks archive ratio or upload rate")
		}
		if profile.Strategy == "gateway" {
			if profile.IngressArchiveBytesPerSecond <= 0 {
				return fmt.Errorf("gateway profile requires measured ingress rate")
			}
			if profile.IngressIncludesSourcePreparation {
				if profile.SnapshotSecondsPerByte != 0 || profile.SeparatedIngressServiceSource != "" {
					return fmt.Errorf("combined ingress cannot also charge separate source preparation")
				}
			} else if profile.SeparatedIngressServiceSource == "" {
				return fmt.Errorf("gateway separated ingress requires queue-free service provenance")
			}
		}
	}
	seen = map[string]bool{}
	for _, profile := range m.MergeProfiles {
		if seen[profile.TargetClass] || profile.TargetClass == "" || profile.Source == "" || profile.Samples < 1 || !validNonnegative(profile.CommandOverheadSeconds, profile.CheckSecondsPerInputByte, profile.CopySecondsPerUniqueByte, profile.VerifySecondsPerUniqueByte) {
			return fmt.Errorf("invalid or duplicate merge profile %q", profile.TargetClass)
		}
		seen[profile.TargetClass] = true
	}
	seen = map[string]bool{}
	for _, p := range m.InputObservationProfiles {
		if p.TargetClass == "" || p.Source == "" || p.Samples < 1 || seen[p.TargetClass] || !validNonnegative(p.FixedSeconds, p.SecondsPerUniqueByte) {
			return fmt.Errorf("invalid input observation profile %q", p.TargetClass)
		}
		seen[p.TargetClass] = true
	}
	seen = map[string]bool{}
	for _, p := range m.PublicationProfiles {
		if p.TargetClass == "" || p.Source == "" || p.Samples < 1 || seen[p.TargetClass] || !validNonnegative(p.FixedSeconds, p.ScanSecondsPerWorkspaceByte, p.CleanupSecondsPerInputByte) {
			return fmt.Errorf("invalid publication profile %q", p.TargetClass)
		}
		seen[p.TargetClass] = true
	}
	return m.validateAdmissionProfiles()
}

func (m Model) validateAdmissionProfiles() error {
	seen := map[string]bool{}
	for _, p := range m.AdmissionProfiles {
		key := p.TargetClass + "\x00" + p.RuntimeID
		if p.Source == "" || p.Samples < 1 || p.TargetClass == "" || p.RuntimeID == "" || seen[key] || !validNonnegative(p.FixedSeconds) {
			return fmt.Errorf("invalid runtime admission profile")
		}
		seen[key] = true
	}
	return nil
}

func (m Model) AdmissionSeconds(targetID string) (float64, error) {
	if err := m.validateAdmissionProfiles(); err != nil {
		return 0, err
	}
	r, ok := m.Resources[targetID]
	if !ok {
		return 0, fmt.Errorf("admission resource missing")
	}
	for _, p := range m.AdmissionProfiles {
		if p.TargetClass == r.Class && p.RuntimeID == r.ExecutionRuntimeID {
			return p.FixedSeconds, nil
		}
	}
	return 0, nil
}

func (m Model) PublicationSeconds(targetID string, workspaceBytes, inputBytes int64) (float64, error) {
	if workspaceBytes < 0 || inputBytes < 0 {
		return 0, fmt.Errorf("negative publication volume")
	}
	for _, p := range m.PublicationProfiles {
		if p.TargetClass == m.Resources[targetID].Class {
			seconds := p.FixedSeconds + float64(workspaceBytes)*p.ScanSecondsPerWorkspaceByte + float64(inputBytes)*p.CleanupSecondsPerInputByte
			if !validNonnegative(seconds) {
				return 0, fmt.Errorf("publication estimate overflow")
			}
			return seconds, nil
		}
	}
	return 0, nil
}

// PreparationSeconds charges the consumer's unique input scan once, never
// once per predecessor. Absence of an evidenced profile preserves legacy data.
func (m Model) PreparationSeconds(targetID string, totalBytes, uniqueBytes int64) (float64, error) {
	merge, err := m.MergeSeconds(targetID, totalBytes, uniqueBytes)
	if err != nil {
		return 0, err
	}
	for _, p := range m.InputObservationProfiles {
		if p.TargetClass == m.Resources[targetID].Class {
			seconds := merge + p.FixedSeconds + float64(uniqueBytes)*p.SecondsPerUniqueByte
			if !validNonnegative(seconds) {
				return 0, fmt.Errorf("input observation estimate overflow")
			}
			return seconds, nil
		}
	}
	return merge, nil
}

func (m Model) Route(sourceID, targetID string, workspaceBytes int64) (Route, error) {
	if err := m.Validate(); err != nil {
		return Route{}, err
	}
	if workspaceBytes < 0 {
		return Route{}, fmt.Errorf("negative workspace bytes")
	}
	source, sourceOK := m.Resources[sourceID]
	target, targetOK := m.Resources[targetID]
	if !sourceOK || !targetOK || source.Domain == "relay" || target.Domain == "relay" {
		return Route{}, fmt.Errorf("materialization route resource missing or invalid")
	}
	result := Route{LogicalBytes: workspaceBytes}
	if sourceID == targetID {
		result.Strategy = "runtime-local"
		return result, nil
	}
	strategy := "gateway"
	if source.Domain == "cloud" && target.Domain == "cloud" {
		strategy = "direct-runtime"
	}
	if source.Domain == "hpc" && target.Domain == "hpc" && source.SharedStorageProven && target.SharedStorageProven && source.SharedStorageID == target.SharedStorageID {
		strategy = "shared-storage"
	}
	var selected *Profile
	for index := range m.Profiles {
		profile := &m.Profiles[index]
		if profile.Strategy == strategy && profile.SourceClass == source.Class && profile.TargetClass == target.Class {
			selected = profile
			break
		}
	}
	if selected == nil {
		return result, fmt.Errorf("no evidenced %s profile for %s -> %s", strategy, source.Class, target.Class)
	}
	p := *selected
	result.Strategy = strategy
	bytes := float64(workspaceBytes)
	if strategy == "shared-storage" {
		result.Stages = []Stage{{Name: "shared-storage-copy-and-verify", SourceResourceID: sourceID, TargetResourceID: targetID, BeforeSeconds: p.ProbeSeconds + bytes*p.SnapshotSecondsPerByte, AfterSeconds: bytes * p.ReceiverSecondsPerByte}}
		return m.withBulkServices(result), nil
	}
	compressed := math.Ceil(bytes * p.ArchiveToWorkspaceRatio)
	// Reserve room for both relay hops without overflowing integer counters.
	if compressed >= float64(math.MaxInt64/2) {
		return result, fmt.Errorf("archive byte estimate overflow")
	}
	archiveBytes := int64(compressed)
	if strategy == "direct-runtime" {
		result.NetworkBytes = archiveBytes
		result.Stages = []Stage{{Name: "peer-snapshot-upload-and-extract", SourceResourceID: sourceID, TargetResourceID: targetID, Bytes: archiveBytes, EffectiveBytesPerSecond: p.UploadArchiveBytesPerSecond, BeforeSeconds: p.ProbeSeconds + bytes*p.SnapshotSecondsPerByte, AfterSeconds: bytes * p.ReceiverSecondsPerByte}}
		if source.BulkServiceGroupID != "" || target.BulkServiceGroupID != "" {
			// The source SSH command uploads through the private peer route;
			// receiver verification is a separate command on the target session.
			result.Stages[0].Name = "peer-snapshot-and-upload"
			result.Stages[0].AfterSeconds = 0
			result.Stages = append(result.Stages, Stage{Name: "peer-receiver-verify-and-extract", SourceResourceID: targetID, TargetResourceID: targetID, BeforeSeconds: bytes * p.ReceiverSecondsPerByte})
		}
	} else {
		result.NetworkBytes = 2 * archiveBytes
		result.RelayNetworkBytes = result.NetworkBytes
		result.Stages = []Stage{
			{Name: "relay-pack-and-receive", SourceResourceID: sourceID, TargetResourceID: m.RelayResourceID, Bytes: archiveBytes, EffectiveBytesPerSecond: p.IngressArchiveBytesPerSecond, BeforeSeconds: p.ProbeSeconds + bytes*p.SnapshotSecondsPerByte, AfterSeconds: bytes * p.ValidationSecondsPerByte},
			{Name: "relay-upload-and-extract", SourceResourceID: m.RelayResourceID, TargetResourceID: targetID, Bytes: archiveBytes, EffectiveBytesPerSecond: p.UploadArchiveBytesPerSecond, AfterSeconds: bytes * p.ReceiverSecondsPerByte},
		}
		if !p.IngressIncludesSourcePreparation {
			result.Stages[0].Name = "relay-source-preparation-and-ingress"
		}
	}
	return m.withBulkServices(result), nil
}

func (m Model) withBulkServices(route Route) Route {
	for i := range route.Stages {
		stage := &route.Stages[i]
		owner := stage.SourceResourceID
		if route.Strategy == "gateway" && i == 1 {
			owner = stage.TargetResourceID
		}
		r := m.Resources[owner]
		stage.ServiceGroupID, stage.ServiceSlots = r.BulkServiceGroupID, r.BulkServiceSlots
	}
	return route
}

// MergeSeconds is charged once per consumer after every dependency completes.
// Checking scans all parents; copy and final verification use the unique union.
func (m Model) MergeSeconds(targetID string, inputTotalBytes, inputUniqueBytes int64) (float64, error) {
	if err := m.Validate(); err != nil {
		return 0, err
	}
	if inputTotalBytes < 0 || inputUniqueBytes < 0 || inputUniqueBytes > inputTotalBytes {
		return 0, fmt.Errorf("invalid merge volume")
	}
	resource, exists := m.Resources[targetID]
	if !exists {
		return 0, fmt.Errorf("merge resource missing")
	}
	if inputTotalBytes == 0 {
		return 0, nil
	}
	for _, profile := range m.MergeProfiles {
		if profile.TargetClass == resource.Class {
			seconds := profile.CommandOverheadSeconds + float64(inputTotalBytes)*profile.CheckSecondsPerInputByte + float64(inputUniqueBytes)*(profile.CopySecondsPerUniqueByte+profile.VerifySecondsPerUniqueByte)
			if !validNonnegative(seconds) {
				return 0, fmt.Errorf("merge estimate overflow")
			}
			return seconds, nil
		}
	}
	return 0, fmt.Errorf("no evidenced merge profile for %s", resource.Class)
}
