package preflight

import (
	"context"
	"time"

	domainpreflight "github.com/UFFeScience/akoflow/internal/domain/preflight"
	"github.com/UFFeScience/akoflow/internal/operationerror"
)

type Probe struct {
	Code, Stage, EntityID, Path string
	Run                         func(context.Context) (string, string, string)
}

func Evaluate(ctx context.Context, id, hash, mode string, probes []Probe) domainpreflight.Report {
	started := time.Now().UTC()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	report := domainpreflight.Report{ID: id, ContextHash: hash, Mode: mode, Ready: true, CreatedAt: started, ExpiresAt: started.Add(2 * time.Minute), Checks: []domainpreflight.Check{}}
	for _, probe := range probes {
		checkCtx, stop := context.WithTimeout(ctx, 5*time.Second)
		status, message, action := "unverified", "Check could not complete before its deadline.", "Refresh validation before starting."
		if checkCtx.Err() == nil {
			status, message, action = probe.Run(checkCtx)
			if checkCtx.Err() != nil {
				status, message, action = "blocked", "Prerequisite check timed out or was cancelled.", "Restore connectivity and run validation again."
			}
		} else {
			status = "blocked"
		}
		stop()
		report.Checks = append(report.Checks, domainpreflight.Check{
			Code: probe.Code, Stage: probe.Stage, EntityID: probe.EntityID,
			Path: probe.Path, Status: status, Message: operationerror.Sanitize(message),
			NextAction: action, ObservedAt: time.Now().UTC(),
		})
		if status == "blocked" {
			report.Ready = false
		}
		if status == "warning" || status == "unverified" {
			report.Warnings++
		}
	}
	return report
}
