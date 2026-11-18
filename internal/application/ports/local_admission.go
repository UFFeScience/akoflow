package ports

import (
	"context"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type LocalActivityCapacity struct {
	CPU         float64
	MemoryBytes int64
}

type LocalReconciliation struct {
	TaskID, RunID, ActivityID  string
	Attempt                    int
	Docker                     bool
	ProcessID, ProcessIdentity string
}

type LocalAdmissionStore interface {
	ReserveLocalActivity(context.Context, *domain.TaskExecution, LocalActivityCapacity, LocalActivityCapacity) (bool, error)
	MarkLocalActivityLaunching(context.Context, string, string, int, bool) error
	RecoverLocalPreparations(context.Context) error
	ListLocalReconciliations(context.Context) ([]LocalReconciliation, error)
	ReleaseReconciledLocalActivity(context.Context, string) error
}
