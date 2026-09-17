package execution

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
)

func TestMetricStorageIsInstalledByCanonicalBootstrap(t *testing.T) {
	t.Setenv("AKOFLOW_SQLITE_JOURNAL_MODE", "DELETE")
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "existing.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := database.Validate(ctx, db); err != nil {
		t.Fatalf("canonical schema with metrics is invalid: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatalf("reopening existing database failed: %v", err)
	}
	repository := New(db)
	samples := []domain.ActivityMetricSample{
		{RunID: "run", ActivityID: "task", Attempt: 1, ObservedAt: 10, CPUSeconds: 1, MemoryBytes: 1024, ReadBytes: 10, Source: "cgroup-v2", Scope: "activity"},
		{RunID: "run", ActivityID: "task", Attempt: 1, ObservedAt: 20, CPUSeconds: 6, MemoryBytes: 2048, ReadBytes: 50, Source: "cgroup-v2", Scope: "activity"},
	}
	if err := repository.SaveActivityMetrics(ctx, samples); err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveActivityMetrics(ctx, samples); err != nil {
		t.Fatal(err)
	}
	summaries, err := repository.ListActivityMetricSummaries(ctx, "run")
	if err != nil || len(summaries) != 1 || summaries[0].Samples != 2 || summaries[0].AverageCPUCores != .5 || summaries[0].PeakMemoryBytes != 2048 {
		t.Fatalf("summaries=%+v err=%v", summaries, err)
	}
	selected, err := repository.GetActivityMetricSummary(ctx, "run", "task", 1)
	if err != nil || selected == nil || selected.Samples != 2 {
		t.Fatalf("selected summary=%+v err=%v", selected, err)
	}
	series, err := repository.ListActivityMetricSamples(ctx, "run", "task", 1)
	if err != nil || len(series) != 2 {
		t.Fatalf("series=%+v err=%v", series, err)
	}
}

func TestMetricQueryOnOlderReadOnlySnapshotIsEmpty(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "snapshot.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	db.Close()
	readonly, err := database.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer readonly.Close()
	repository := New(readonly)
	if values, err := repository.ListActivityMetricSummaries(ctx, "run"); err != nil || len(values) != 0 {
		t.Fatalf("summaries=%+v err=%v", values, err)
	}
	if values, err := repository.ListActivityMetricSamples(ctx, "run", "task", 1); err != nil || len(values) != 0 {
		t.Fatalf("series=%+v err=%v", values, err)
	}
	if value, err := repository.GetActivityMetricSummary(ctx, "run", "task", 1); err != nil || value != nil {
		t.Fatalf("summary=%+v err=%v", value, err)
	}
}

func TestMetricSeriesIsBoundedAndKeepsLastSample(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(filepath.Join(t.TempDir(), "metrics.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	repository := New(db)
	samples := make([]domain.ActivityMetricSample, 3000)
	for index := range samples {
		samples[index] = domain.ActivityMetricSample{RunID: "run", ActivityID: "task", Attempt: 1,
			ObservedAt: float64(index + 1), CPUSeconds: float64(index), Source: "test", Scope: "activity"}
	}
	if err := repository.SaveActivityMetrics(ctx, samples); err != nil {
		t.Fatal(err)
	}
	series, err := repository.ListActivityMetricSamples(ctx, "run", "task", 1)
	if err != nil || len(series) > 1200 || series[len(series)-1].ObservedAt != 3000 {
		t.Fatalf("length=%d last=%+v err=%v", len(series), series[len(series)-1], err)
	}
}
