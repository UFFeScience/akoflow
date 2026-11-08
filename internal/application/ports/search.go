package ports

import (
	"context"
	domainsearch "github.com/UFFeScience/akoflow/internal/domain/search"
)

type GlobalSearch interface {
	Search(context.Context, domainsearch.Query) (domainsearch.Page, error)
}
