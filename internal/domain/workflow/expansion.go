package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	ExpansionStatusApplied  = "applied"
	ExpansionStatusRejected = "rejected"
	DefaultMaxActivities    = 1000
	DefaultMaxDependencies  = 4000
)

// ExpansionRequest is the immutable output contract emitted by a dynamic
// activity. SourceEventID is the replay key; Item.Key is stable within it.
type ExpansionRequest struct {
	ID                string                `json:"id"`
	WorkflowVersionID string                `json:"workflowVersionId"`
	ExecutionRunID    string                `json:"executionRunId,omitempty"`
	SourceActivityID  string                `json:"sourceActivityId"`
	SourceEventID     string                `json:"sourceEventId"`
	Sequence          int                   `json:"sequence"`
	Activities        []ExpansionActivity   `json:"activities"`
	Dependencies      []ExpansionDependency `json:"dependencies,omitempty"`
	Metadata          map[string]any        `json:"metadata,omitempty"`
}

type ExpansionActivity struct {
	Key      string   `json:"key"`
	Activity Activity `json:"activity"`
}

type ExpansionDependency struct {
	ActivityKey          string `json:"activityKey,omitempty"`
	ActivityID           string `json:"activityId,omitempty"`
	DependsOnActivityKey string `json:"dependsOnActivityKey,omitempty"`
	DependsOnActivityID  string `json:"dependsOnActivityId,omitempty"`
	Type                 string `json:"type,omitempty"`
}

// WorkflowExpansion is an append-only decision record. ResultRevision is a
// projection revision and never a replacement WorkflowVersion.
type WorkflowExpansion struct {
	ID                string               `json:"id"`
	WorkflowVersionID string               `json:"workflowVersionId"`
	ExecutionRunID    string               `json:"executionRunId,omitempty"`
	SourceActivityID  string               `json:"sourceActivityId"`
	SourceEventID     string               `json:"sourceEventId"`
	Sequence          int                  `json:"sequence"`
	ResultRevision    int                  `json:"resultRevision"`
	Status            string               `json:"status"`
	FailureReason     string               `json:"failureReason,omitempty"`
	Activities        []Activity           `json:"activities"`
	Dependencies      []ActivityDependency `json:"dependencies,omitempty"`
	Metadata          map[string]any       `json:"metadata,omitempty"`
	CreatedAt         time.Time            `json:"createdAt"`
}

type ExpandedWorkflow struct {
	WorkflowVersionID string               `json:"workflowVersionId"`
	Revision          int                  `json:"revision"`
	Activities        []Activity           `json:"activities"`
	Dependencies      []ActivityDependency `json:"dependencies"`
	Expansions        []WorkflowExpansion  `json:"expansions"`
}

type ExpansionLimits struct {
	MaxActivities   int `json:"maxActivities"`
	MaxDependencies int `json:"maxDependencies"`
}

func (r ExpansionRequest) Validate() error {
	if r.ID == "" || r.WorkflowVersionID == "" || r.SourceActivityID == "" || r.SourceEventID == "" {
		return fmt.Errorf("expansion id, workflow version, source activity and source event are required")
	}
	if r.Sequence < 1 {
		return fmt.Errorf("expansion sequence must be positive")
	}
	if len(r.Activities) == 0 {
		return fmt.Errorf("expansion must materialize at least one activity")
	}
	seen := map[string]bool{}
	for _, item := range r.Activities {
		if strings.TrimSpace(item.Key) == "" {
			return fmt.Errorf("expansion activity key is required")
		}
		if seen[item.Key] {
			return fmt.Errorf("duplicate expansion activity key %q", item.Key)
		}
		seen[item.Key] = true
	}
	return nil
}

func DeterministicExpansionID(workflowVersionID, sourceEventID string) string {
	return deterministicID("wfx", workflowVersionID, sourceEventID)
}

func DeterministicActivityID(workflowVersionID, sourceEventID, key string) string {
	return deterministicID("wfa", workflowVersionID, sourceEventID, key)
}

func deterministicID(prefix string, parts ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return prefix + "_" + hex.EncodeToString(hash[:16])
}

func MaterializeExpansion(base WorkflowVersion, prior []WorkflowExpansion, request ExpansionRequest, limits ExpansionLimits) (WorkflowExpansion, ExpandedWorkflow, error) {
	if err := request.Validate(); err != nil {
		return WorkflowExpansion{}, ExpandedWorkflow{}, err
	}
	if request.WorkflowVersionID != base.ID {
		return WorkflowExpansion{}, ExpandedWorkflow{}, fmt.Errorf("expansion workflow version does not match immutable definition")
	}
	if limits.MaxActivities <= 0 {
		limits.MaxActivities = DefaultMaxActivities
	}
	if limits.MaxDependencies <= 0 {
		limits.MaxDependencies = DefaultMaxDependencies
	}
	result := expansionProjection(base, prior)
	existing := map[string]bool{}
	for _, activity := range result.Activities {
		existing[activity.ID] = true
	}
	if !existing[request.SourceActivityID] {
		return WorkflowExpansion{}, ExpandedWorkflow{}, fmt.Errorf("source activity %q is not part of the resulting graph", request.SourceActivityID)
	}
	materialized, keys, err := materializeActivities(base.ID, request, existing)
	if err != nil {
		return WorkflowExpansion{}, ExpandedWorkflow{}, err
	}
	dependencies, err := materializeDependencies(request.Dependencies, keys, existing)
	if err != nil {
		return WorkflowExpansion{}, ExpandedWorkflow{}, err
	}
	result.Activities = append(result.Activities, materialized...)
	result.Dependencies = append(result.Dependencies, dependencies...)
	if len(result.Activities) > limits.MaxActivities || len(result.Dependencies) > limits.MaxDependencies {
		return WorkflowExpansion{}, ExpandedWorkflow{}, fmt.Errorf("expanded workflow exceeds limits (%d activities, %d dependencies)", limits.MaxActivities, limits.MaxDependencies)
	}
	if hasDependencyCycle(result.Activities, result.Dependencies) {
		return WorkflowExpansion{}, ExpandedWorkflow{}, fmt.Errorf("expansion introduces a dependency cycle")
	}
	expansion := WorkflowExpansion{
		ID:                DeterministicExpansionID(base.ID, request.SourceEventID),
		WorkflowVersionID: base.ID, ExecutionRunID: request.ExecutionRunID,
		SourceActivityID: request.SourceActivityID, SourceEventID: request.SourceEventID,
		Sequence: request.Sequence, ResultRevision: result.Revision + 1,
		Status: ExpansionStatusApplied, Activities: materialized,
		Dependencies: dependencies, Metadata: request.Metadata,
	}
	result.Revision, result.Expansions = expansion.ResultRevision, append(result.Expansions, expansion)
	return expansion, result, nil
}

func expansionProjection(base WorkflowVersion, prior []WorkflowExpansion) ExpandedWorkflow {
	result := ExpandedWorkflow{WorkflowVersionID: base.ID, Activities: append([]Activity{}, base.Activities...), Dependencies: append([]ActivityDependency{}, base.Dependencies...), Expansions: append([]WorkflowExpansion{}, prior...)}
	for _, expansion := range prior {
		if expansion.Status != ExpansionStatusApplied {
			continue
		}
		result.Revision++
		result.Activities = append(result.Activities, expansion.Activities...)
		result.Dependencies = append(result.Dependencies, expansion.Dependencies...)
	}
	return result
}

func materializeActivities(versionID string, request ExpansionRequest, existing map[string]bool) ([]Activity, map[string]string, error) {
	keys, materialized := map[string]string{}, make([]Activity, 0, len(request.Activities))
	for _, item := range request.Activities {
		activity := item.Activity
		activity.ID, activity.WorkflowVersionID = DeterministicActivityID(versionID, request.SourceEventID, item.Key), versionID
		if activity.ExternalID == "" {
			activity.ExternalID = item.Key
		}
		if existing[activity.ID] {
			return nil, nil, fmt.Errorf("activity id collision %q", activity.ID)
		}
		if err := activity.Validate(); err != nil {
			return nil, nil, err
		}
		keys[item.Key], existing[activity.ID] = activity.ID, true
		materialized = append(materialized, activity)
	}
	return materialized, keys, nil
}

func materializeDependencies(items []ExpansionDependency, keys map[string]string, existing map[string]bool) ([]ActivityDependency, error) {
	resolve := func(key, id string) string {
		if key != "" {
			return keys[key]
		}
		return id
	}
	dependencies := make([]ActivityDependency, 0, len(items))
	for _, item := range items {
		dependency := ActivityDependency{ActivityID: resolve(item.ActivityKey, item.ActivityID), DependsOnActivityID: resolve(item.DependsOnActivityKey, item.DependsOnActivityID), Type: item.Type}
		if dependency.Type == "" {
			dependency.Type = "control"
		}
		if !existing[dependency.ActivityID] || !existing[dependency.DependsOnActivityID] {
			return nil, fmt.Errorf("expansion dependency references an unknown activity")
		}
		if dependency.ActivityID == dependency.DependsOnActivityID {
			return nil, fmt.Errorf("expansion dependency cannot reference itself")
		}
		dependencies = append(dependencies, dependency)
	}
	return dependencies, nil
}

func hasDependencyCycle(activities []Activity, dependencies []ActivityDependency) bool {
	degree, next := map[string]int{}, map[string][]string{}
	for _, activity := range activities {
		degree[activity.ID] = 0
	}
	for _, dependency := range dependencies {
		degree[dependency.ActivityID]++
		next[dependency.DependsOnActivityID] = append(next[dependency.DependsOnActivityID], dependency.ActivityID)
	}
	ready := make([]string, 0)
	for id, value := range degree {
		if value == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)
	visited := 0
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		visited++
		for _, child := range next[id] {
			degree[child]--
			if degree[child] == 0 {
				ready = append(ready, child)
			}
		}
	}
	return visited != len(degree)
}
