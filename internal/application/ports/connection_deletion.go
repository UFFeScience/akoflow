package ports

import "context"

type ConnectionDependency struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ConnectionDeletion struct {
	Name         string                 `json:"name"`
	Dependencies []ConnectionDependency `json:"dependencies"`
}

type ConnectionDeletionStore interface {
	PreviewConnectionDeletion(context.Context, string) (ConnectionDeletion, error)
	DeleteConnection(context.Context, string) (ConnectionDeletion, error)
}
