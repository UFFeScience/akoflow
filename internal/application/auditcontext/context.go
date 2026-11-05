// Package auditcontext carries server-established request provenance across
// adapters without a global registry or infrastructure dependency.
package auditcontext

import "context"

type key struct{}
type Trace struct {
	RequestID     string
	CorrelationID string
	ActorID       string
	ActorType     string
	AuthOutcome   string
}

func With(ctx context.Context, trace *Trace) context.Context {
	return context.WithValue(ctx, key{}, trace)
}
func From(ctx context.Context) *Trace { trace, _ := ctx.Value(key{}).(*Trace); return trace }
func Identify(ctx context.Context, actor, kind, outcome string) {
	if trace := From(ctx); trace != nil {
		trace.ActorID, trace.ActorType, trace.AuthOutcome = actor, kind, outcome
	}
}
