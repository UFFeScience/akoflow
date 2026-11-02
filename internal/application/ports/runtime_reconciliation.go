package ports

// RuntimeReconciliationPending marks a startup whose external outcome is not
// yet proven terminal. Such a handle must retain runtime cleanup ownership.
const RuntimeReconciliationPending = "runtimeReconciliationPending"

// RuntimeReconciliationFailure carries bounded, non-sensitive correlation
// metadata when a runtime cannot establish the outcome of a startup request.
type RuntimeReconciliationFailure interface {
	error
	RuntimeReconciliationMetadata() map[string]any
}

type runtimeReconciliationError struct {
	cause    error
	metadata map[string]any
}

func NewRuntimeReconciliationError(cause error, metadata map[string]any) error {
	copy := make(map[string]any, len(metadata)+1)
	for key, value := range metadata {
		copy[key] = value
	}
	copy[RuntimeReconciliationPending] = true
	return &runtimeReconciliationError{cause: cause, metadata: copy}
}

func (e *runtimeReconciliationError) Error() string {
	if e.cause == nil {
		return "runtime outcome requires reconciliation"
	}
	return e.cause.Error()
}
func (e *runtimeReconciliationError) Unwrap() error { return e.cause }
func (e *runtimeReconciliationError) RuntimeReconciliationMetadata() map[string]any {
	copy := make(map[string]any, len(e.metadata))
	for key, value := range e.metadata {
		copy[key] = value
	}
	return copy
}
