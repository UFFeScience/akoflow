package ports

import "errors"

var ErrExecutionSubmissionConflict = errors.New("execution submission key or run ID is already in use with a different request")
