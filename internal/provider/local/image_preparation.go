package local

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainexecution "github.com/UFFeScience/akoflow/internal/domain/execution"
	"github.com/google/uuid"
)

func (a *Adapter) prepareImage(ctx context.Context, execution domain.ActivityExecutionContext, image, platform string) ([]byte, error) {
	start := time.Now().UTC()
	output, err := exec.CommandContext(ctx, "docker", "image", "inspect", "--format", "{{.Os}}/{{.Architecture}}", image).CombinedOutput()
	if err == nil && strings.TrimSpace(string(output)) == platform {
		observation := newImageObservation(execution, domainexecution.ImagePreparationCacheHit, start)
		end := time.Now().UTC()
		observation.Status, observation.FinishedAt = "completed", &end
		return nil, a.savePreparation(ctx, execution.Run.ID, observation)
	}
	if err == nil && strings.HasPrefix(image, "sha256:") {
		return []byte("local image platform does not match requested platform"), errors.New("local image platform mismatch")
	}
	if err != nil && !strings.Contains(strings.ToLower(string(output)), "no such image") {
		return output, err
	}
	return a.pullImage(ctx, execution, image, platform)
}

func (a *Adapter) pullImage(ctx context.Context, execution domain.ActivityExecutionContext, image, platform string) ([]byte, error) {
	key := imagePullKey{execution.Run.ID, image, platform}
	a.pullsMu.Lock()
	if pull := a.pulls[key]; pull != nil {
		a.pullsMu.Unlock()
		return a.waitForPull(ctx, execution, pull)
	}
	observation := newImageObservation(execution, domainexecution.ImagePreparationPull, time.Now().UTC())
	pull := &imagePull{operationID: observation.OperationID, done: make(chan struct{})}
	if a.pulls == nil {
		a.pulls = make(map[imagePullKey]*imagePull)
	}
	a.pulls[key] = pull
	a.pullsMu.Unlock()

	defer func() {
		a.pullsMu.Lock()
		delete(a.pulls, key)
		close(pull.done)
		a.pullsMu.Unlock()
	}()
	if err := a.savePreparation(ctx, execution.Run.ID, observation); err != nil {
		pull.err = err
		pull.status = "failed"
		pull.finishedAt = time.Now().UTC()
		return nil, err
	}
	output, commandErr := exec.CommandContext(ctx, "docker", "pull", "--platform", platform, image).CombinedOutput()
	end := time.Now().UTC()
	observation.FinishedAt, observation.Status = &end, "completed"
	if ctx.Err() != nil {
		observation.Status = "cancelled"
	} else if commandErr != nil {
		observation.Status = "failed"
	}
	pull.output, pull.finishedAt, pull.status = output, end, observation.Status
	pull.err = errors.Join(commandErr, a.savePreparation(ctx, execution.Run.ID, observation))
	return output, pull.err
}

func (a *Adapter) savePreparation(ctx context.Context, runID string, observation domainexecution.ImagePreparationObservation) error {
	persist, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	return a.preparation.RecordImagePreparation(persist, runID, observation)
}

func newImageObservation(execution domain.ActivityExecutionContext, kind domainexecution.ImagePreparationKind, start time.Time) domainexecution.ImagePreparationObservation {
	return domainexecution.ImagePreparationObservation{
		OperationID: uuid.NewString(), ActivityID: execution.Activity.ID,
		Attempt: max(1, execution.Attempt), Kind: kind, Status: "running", StartedAt: start,
	}
}
