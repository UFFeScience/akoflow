package ports

import (
	"context"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type WorkflowStore interface {
	Create(context.Context, domain.WorkflowDefinition) error
	List(context.Context) ([]domain.WorkflowDefinition, error)
	Find(context.Context, string) (*domain.WorkflowDefinition, error)
	FindVersion(context.Context, string) (*domain.WorkflowVersion, error)
}

type EnvironmentCatalog interface {
	Create(context.Context, domain.EnvironmentDefinition) error
	Replace(context.Context, domain.EnvironmentDefinition) error
	Delete(context.Context, string) error
	List(context.Context) ([]domain.EnvironmentDefinition, error)
	Find(context.Context, string) (*domain.EnvironmentDefinition, error)
	EnvironmentStore
}

type PlanStore interface {
	Save(context.Context, domain.SchedulePlan) error
	List(context.Context) ([]domain.SchedulePlan, error)
	Find(context.Context, string) (*domain.SchedulePlan, error)
}

type DataCatalog interface {
	CatalogArtifacts(context.Context, domain.ActivityHandle) error
	ListInstances(context.Context, string) ([]domain.DataObjectInstance, error)
	ListLocations(context.Context, string) ([]domain.DataLocation, error)
	ListArtifacts(context.Context) ([]domain.ExecutableArtifact, error)
	ListArtifactLocations(context.Context) ([]domain.ArtifactLocation, error)
	ListArtifactMaterializations(context.Context, string) ([]domain.ArtifactMaterialization, error)
	ListArtifactTransferRuns(context.Context, string) ([]domain.DataTransferRun, error)
	SaveArtifactMaterialization(context.Context, domain.ArtifactMaterialization) error
	SaveTransferRun(context.Context, domain.DataTransferRun) error
	FindTransferRun(context.Context, string) (*domain.DataTransferRun, error)
	SaveArtifactBuild(context.Context, domain.ArtifactBuild) error
	FindArtifactBuildByCacheKey(context.Context, string) (*domain.ArtifactBuild, error)
	FindArtifactBuild(context.Context, string) (*domain.ArtifactBuild, error)
	ListArtifactBuilds(context.Context, string) ([]domain.ArtifactBuild, error)
	SaveBuildRun(context.Context, domain.BuildRun) error
	FindBuildRun(context.Context, string) (*domain.BuildRun, error)
	ListBuildRuns(context.Context, string) ([]domain.BuildRun, error)
	SaveBuildContext(context.Context, domain.BuildContextArtifact) error
	FindBuildContext(context.Context, string) (*domain.BuildContextArtifact, error)
	PublishBuildOutput(context.Context, string, domain.ArtifactVariant, domain.ArtifactLocation) error
	FindBuildOutput(context.Context, string) (*domain.ArtifactVariant, *domain.ArtifactLocation, error)
	FindDockerBuildOutput(context.Context, string, string) (*domain.ArtifactBuild, *domain.ArtifactVariant, *domain.ArtifactLocation, error)
	RegisterArtifactVersion(context.Context, domain.ArtifactVersion) error
}
