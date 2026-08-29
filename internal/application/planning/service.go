package planning

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

const EventPlanningSessionRequested = "planning.session.requested"

type SchedulerRegistry interface {
	Find(string) (ports.Scheduler, bool)
	List() []ports.SchedulerDescriptor
}

type Coordinator struct {
	Store        ports.PlanningStore
	Plans        ports.PlanStore
	Workflows    ports.WorkflowStore
	Environments ports.EnvironmentCatalog
	Resources    ports.ResourceInventory
	Scopes       ports.ExecutionScopeStore
	Topologies   ports.NetworkTopologyStore
	Validator    ports.PlanValidator
	Registry     SchedulerRegistry
	Events       ports.EventPublisher
}

func (s *Coordinator) Algorithms() []ports.SchedulerDescriptor { return s.Registry.List() }

func (s *Coordinator) Create(
	ctx context.Context,
	session domain.PlanningSession,
) (*domain.PlanningSession, error) {
	if session.ID == "" || session.WorkflowVersionID == "" || session.ExecutionScopeID == "" || session.NetworkTopologyID == "" {
		return nil, fmt.Errorf("planning session id, workflow version, execution scope and topology are required")
	}
	if len(session.Algorithms) == 0 {
		return nil, fmt.Errorf("select at least one planning algorithm")
	}
	seen := map[string]bool{}
	runs := make([]domain.AlgorithmRun, 0, len(session.Algorithms))
	for _, selection := range session.Algorithms {
		scheduler, ok := s.Registry.Find(selection.ID)
		if !ok {
			return nil, fmt.Errorf("planning algorithm %q is not registered", selection.ID)
		}
		if seen[selection.ID] {
			return nil, fmt.Errorf("planning algorithm %q is duplicated", selection.ID)
		}
		seen[selection.ID] = true
		descriptor := scheduler.Descriptor()
		runs = append(runs, domain.AlgorithmRun{
			ID:                session.ID + "-" + selection.ID,
			PlanningSessionID: session.ID,
			Algorithm:         selection.ID,
			Objective:         descriptor.Objective,
			Status:            domain.PlanningStatusQueued,
			Configuration:     selection.Configuration,
		})
	}
	if workflow, err := s.Workflows.FindVersion(ctx, session.WorkflowVersionID); err != nil || workflow == nil {
		if err == nil {
			err = fmt.Errorf("workflow version not found")
		}
		return nil, err
	}
	if scope, err := s.Scopes.FindScope(ctx, session.ExecutionScopeID); err != nil || scope == nil {
		if err == nil {
			err = fmt.Errorf("execution scope not found")
		}
		return nil, err
	}
	if topology, err := s.Topologies.Find(ctx, session.NetworkTopologyID); err != nil || topology == nil {
		if err == nil {
			err = fmt.Errorf("network topology not found")
		}
		return nil, err
	}
	session.Status = domain.PlanningStatusQueued
	session.CreatedAt = time.Now().UTC()
	if err := s.Store.CreateSession(ctx, session, runs); err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(map[string]string{"planningSessionId": session.ID})
	job, err := domainqueue.New(domainqueue.CategoryPlanning, EventPlanningSessionRequested, payload, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	job.AggregateType = "planning_session"
	job.AggregateID = session.ID
	job.IdempotencyKey = "planning-session:" + session.ID
	if _, err := s.Events.Publish(ctx, job); err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *Coordinator) Execute(ctx context.Context, sessionID string) error {
	session, err := s.Store.FindSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return fmt.Errorf("planning session %q not found", sessionID)
	}
	if session.Status == domain.PlanningStatusCompleted || session.Status == domain.PlanningStatusCancelled {
		return nil
	}
	if err := s.Store.SetSessionRunning(ctx, sessionID); err != nil {
		return err
	}
	request, err := s.buildRequest(ctx, *session)
	if err != nil {
		_ = s.Store.SetSessionFailed(ctx, sessionID, err.Error())
		return err
	}
	runs, err := s.Store.ListAlgorithmRuns(ctx, sessionID)
	if err != nil {
		return err
	}
	completed := 0
	for _, run := range runs {
		if run.Status == domain.PlanningStatusCompleted {
			completed++
			continue
		}
		scheduler, ok := s.Registry.Find(run.Algorithm)
		if !ok {
			_ = s.Store.SetAlgorithmRunFailed(ctx, run.ID, "algorithm is no longer registered")
			continue
		}
		_ = s.Store.SetAlgorithmRunRunning(ctx, run.ID)
		reporter := progressReporter{store: s.Store, sessionID: sessionID, runID: run.ID, completedRuns: completed, totalRuns: len(runs)}
		sink := &candidateSink{store: s.Store, validator: s.Validator, request: request, sessionID: sessionID, run: run}
		if err := scheduler.Schedule(ctx, request, run.Configuration, reporter, sink); err != nil {
			_ = s.Store.SetAlgorithmRunFailed(ctx, run.ID, err.Error())
			continue
		}
		_ = s.Store.SetAlgorithmRunCompleted(ctx, run.ID)
		completed++
		_ = s.Store.UpdateSessionProgress(ctx, sessionID, float64(completed)/float64(len(runs)))
	}
	candidates, err := s.Store.ListCandidates(ctx, sessionID)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		err = fmt.Errorf("no planning algorithm produced a valid candidate")
		_ = s.Store.SetSessionFailed(ctx, sessionID, err.Error())
		return err
	}
	rankCandidates(candidates)
	if err := s.Store.UpdateCandidateRanks(ctx, candidates); err != nil {
		return err
	}
	return s.Store.SetSessionCompleted(ctx, sessionID)
}

func (s *Coordinator) buildRequest(
	ctx context.Context,
	session domain.PlanningSession,
) (domain.PlanningRequest, error) {
	workflow, err := s.Workflows.FindVersion(ctx, session.WorkflowVersionID)
	if err != nil || workflow == nil {
		return domain.PlanningRequest{}, fmt.Errorf("load workflow version: %w", err)
	}
	scope, err := s.Scopes.FindScope(ctx, session.ExecutionScopeID)
	if err != nil || scope == nil {
		return domain.PlanningRequest{}, fmt.Errorf("load execution scope: %w", err)
	}
	topology, err := s.Topologies.Find(ctx, session.NetworkTopologyID)
	if err != nil || topology == nil {
		return domain.PlanningRequest{}, fmt.Errorf("load network topology: %w", err)
	}
	allResources, err := s.Resources.List(ctx)
	if err != nil {
		return domain.PlanningRequest{}, err
	}
	allowed := map[string]bool{}
	for _, id := range scope.EnvironmentVersionIDs {
		allowed[id] = true
	}
	resources := []domain.Resource{}
	for _, resource := range allResources {
		if allowed[resource.EnvironmentVersionID] {
			resources = append(resources, resource)
		}
	}
	definitions, err := s.Environments.List(ctx)
	if err != nil {
		return domain.PlanningRequest{}, err
	}
	environments := []domain.EnvironmentVersion{}
	profiles := []domain.ActivityResourceProfile{}
	for _, definition := range definitions {
		if allowed[definition.Version.ID] {
			environments = append(environments, definition.Version)
			profiles = append(profiles, definition.Profiles...)
		}
	}
	return domain.PlanningRequest{
		Workflow:         *workflow,
		ExecutionScope:   *scope,
		Environments:     environments,
		Resources:        resources,
		NetworkTopology:  *topology,
		ActivityProfiles: profiles,
		DeadlineSeconds:  session.DeadlineSeconds,
		Budget:           session.Budget,
	}, nil
}

func (s *Coordinator) Select(
	ctx context.Context,
	sessionID string,
	candidateID string,
) (*domain.SchedulePlan, error) {
	candidate, err := s.Store.FindCandidate(ctx, candidateID)
	if err != nil {
		return nil, err
	}
	if candidate == nil {
		return nil, fmt.Errorf("planning candidate not found")
	}
	if candidate.PlanningSessionID != sessionID {
		return nil, fmt.Errorf("candidate does not belong to planning session")
	}
	if !candidate.Feasible {
		return nil, fmt.Errorf("an infeasible candidate cannot be selected")
	}
	if err := s.Store.SelectCandidate(ctx, sessionID, *candidate); err != nil {
		return nil, err
	}
	return &candidate.Plan, nil
}

type progressReporter struct {
	store                    ports.PlanningStore
	sessionID, runID         string
	completedRuns, totalRuns int
}

func (r progressReporter) Report(ctx context.Context, progress float64, _ string) error {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	if err := r.store.UpdateAlgorithmRunProgress(ctx, r.runID, progress); err != nil {
		return err
	}
	return r.store.UpdateSessionProgress(ctx, r.sessionID, (float64(r.completedRuns)+progress)/float64(r.totalRuns))
}

type candidateSink struct {
	store     ports.PlanningStore
	validator ports.PlanValidator
	request   domain.PlanningRequest
	sessionID string
	run       domain.AlgorithmRun
	count     int
}

func (s *candidateSink) Emit(ctx context.Context, plan domain.SchedulePlan) error {
	if err := s.validator.Validate(
		plan,
		s.request.Workflow,
		s.request.Resources,
		s.request.ExecutionScope,
		s.request.NetworkTopology,
	); err != nil {
		return fmt.Errorf("validate %s candidate: %w", s.run.Algorithm, err)
	}
	planHash, err := planFingerprint(plan)
	if err != nil {
		return err
	}
	fingerprint := candidateFingerprint(s.run.ID, planHash)
	s.count++
	plan.ID = fmt.Sprintf("%s-%s", s.sessionID, fingerprint[:12])
	for index := range plan.Assignments {
		plan.Assignments[index].PlanID = plan.ID
		plan.Assignments[index].ID = plan.ID + "-" + plan.Assignments[index].ActivityID
	}
	candidate := domain.PlanCandidate{
		ID:                "candidate-" + fingerprint[:20],
		PlanningSessionID: s.sessionID,
		AlgorithmRunID:    s.run.ID,
		Algorithm:         s.run.Algorithm,
		Objective:         s.run.Objective,
		Feasible:          plan.Predicted.Feasible,
		Predicted:         plan.Predicted,
		Plan:              plan,
		Fingerprint:       fingerprint,
		CreatedAt:         time.Now().UTC(),
	}
	return s.store.SaveCandidate(ctx, candidate)
}

func planFingerprint(plan domain.SchedulePlan) (string, error) {
	copy := plan
	copy.ID = ""
	for index := range copy.Assignments {
		copy.Assignments[index].ID = ""
		copy.Assignments[index].PlanID = ""
	}
	data, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func candidateFingerprint(runID, planHash string) string {
	hash := sha256.Sum256([]byte(runID + ":" + planHash))
	return hex.EncodeToString(hash[:])
}

func rankCandidates(items []domain.PlanCandidate) {
	for index := range items {
		items[index].Dominated = false
		items[index].ParetoOptimal = false
	}
	for i := range items {
		for j := range items {
			if i == j {
				continue
			}
			if dominates(items[j], items[i]) {
				items[i].Dominated = true
				break
			}
		}
		items[i].ParetoOptimal = items[i].Feasible && !items[i].Dominated
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Feasible != items[j].Feasible {
			return items[i].Feasible
		}
		if items[i].ParetoOptimal != items[j].ParetoOptimal {
			return items[i].ParetoOptimal
		}
		if items[i].Predicted.MakespanSeconds != items[j].Predicted.MakespanSeconds {
			return items[i].Predicted.MakespanSeconds < items[j].Predicted.MakespanSeconds
		}
		return items[i].Predicted.Cost < items[j].Predicted.Cost
	})
	for index := range items {
		items[index].Rank = index + 1
	}
}
func dominates(left, right domain.PlanCandidate) bool {
	if !left.Feasible {
		return false
	}
	if !right.Feasible {
		return true
	}
	noWorse := left.Predicted.MakespanSeconds <= right.Predicted.MakespanSeconds && left.Predicted.Cost <= right.Predicted.Cost
	strict := left.Predicted.MakespanSeconds < right.Predicted.MakespanSeconds || left.Predicted.Cost < right.Predicted.Cost
	return noWorse && strict
}

func NormalizeAlgorithmID(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
