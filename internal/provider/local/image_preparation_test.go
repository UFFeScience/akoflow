package local

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainexecution "github.com/UFFeScience/akoflow/internal/domain/execution"
)

type imageObservations struct {
	values []domainexecution.ImagePreparationObservation
}

func (r *imageObservations) RecordImagePreparation(_ context.Context, _ string, value domainexecution.ImagePreparationObservation) error {
	r.values = append(r.values, value)
	return nil
}

func TestLocalDockerRecordsAbsentImagePullAndVerifiedCacheHit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a shell for the Docker fixture")
	}
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("IMAGE_MARKER", filepath.Join(dir, "image"))
	script := `#!/bin/sh
case "$1" in
 image)
  if [ -f "$IMAGE_MARKER" ]; then
   echo "linux/amd64"
  else
   echo "No such image" >&2
   exit 1
  fi ;;
 pull) : > "$IMAGE_MARKER" ;;
 *) exit 2 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	recorded := &imageObservations{}
	adapter := NewWithImagePreparation(recorded)
	execution := domain.ActivityExecutionContext{
		Run: domain.ExecutionRun{ID: "run"}, Activity: domain.Activity{ID: "task"}, Attempt: 2,
	}
	for range 2 {
		if output, err := adapter.prepareImage(context.Background(), execution, "fixture:latest", "linux/amd64"); err != nil {
			t.Fatalf("prepare image: %s: %v", output, err)
		}
	}
	events := recorded.values
	if len(events) != 3 ||
		events[0].Kind != domainexecution.ImagePreparationPull || events[0].FinishedAt != nil ||
		events[1].OperationID != events[0].OperationID || events[1].Status != "completed" ||
		events[1].Attempt != 2 || events[1].FinishedAt == nil ||
		events[2].Kind != domainexecution.ImagePreparationCacheHit || events[2].FinishedAt == nil {
		t.Fatalf("unexpected image observations: %+v", events)
	}
}

type sharedPullRecorder struct {
	active chan domainexecution.ImagePreparationObservation
}

func (r sharedPullRecorder) RecordImagePreparation(_ context.Context, _ string, event domainexecution.ImagePreparationObservation) error {
	if event.FinishedAt == nil {
		r.active <- event
	}
	return nil
}

func TestConcurrentLocalActivitiesSharePullOperation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a shell for the Docker fixture")
	}
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("RELEASE_PULL", filepath.Join(dir, "release"))
	t.Setenv("PULL_LOG", filepath.Join(dir, "pulls"))
	script := `#!/bin/sh
printf '%s\n' pull >> "$PULL_LOG"
while [ ! -e "$RELEASE_PULL" ]; do /bin/sleep 0.01; done
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	recorder := sharedPullRecorder{active: make(chan domainexecution.ImagePreparationObservation, 2)}
	adapter := NewWithImagePreparation(recorder)
	done := make(chan error, 2)
	start := func(activity string) {
		go func() {
			execution := domain.ActivityExecutionContext{Run: domain.ExecutionRun{ID: "run"}, Activity: domain.Activity{ID: activity}}
			_, err := adapter.pullImage(ctx, execution, "fixture", "linux/amd64")
			done <- err
		}()
	}
	read := func() domainexecution.ImagePreparationObservation {
		t.Helper()
		select {
		case observation := <-recorder.active:
			return observation
		case <-ctx.Done():
			t.Fatal("pull observation not recorded")
			return domainexecution.ImagePreparationObservation{}
		}
	}
	start("owner")
	owner := read()
	start("waiter")
	waiter := read()
	if owner.Shared || !waiter.Shared || waiter.ActivityID != "waiter" || waiter.OperationID != owner.OperationID {
		t.Fatalf("owner=%+v waiter=%+v", owner, waiter)
	}
	if err := os.WriteFile(filepath.Join(dir, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	log, err := os.ReadFile(filepath.Join(dir, "pulls"))
	if err != nil || string(log) != "pull\n" {
		t.Fatalf("expected one Docker pull, got %q: %v", log, err)
	}
}
