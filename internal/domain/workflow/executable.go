package workflow

import (
	"fmt"
	"strings"
)

type ExecutableSourceType string
type DeliveryStrategy string
type ExecutableFormat string

const (
	ExecutableSourceCatalog             ExecutableSourceType = "catalog"
	ExecutableSourceOCI                 ExecutableSourceType = "oci"
	ExecutableSourceLocalContainerImage ExecutableSourceType = "local-container-image"
	ExecutableSourceBuild               ExecutableSourceType = "build"
	ExecutableSourceLocalFile           ExecutableSourceType = "local-file"
	ExecutableSourceRemoteFile          ExecutableSourceType = "remote-file"
	ExecutableSourceObjectStorage       ExecutableSourceType = "object-storage"
	ExecutableSourceHTTP                ExecutableSourceType = "http"
	DeliveryAuto                        DeliveryStrategy     = "auto"
	DeliveryManaged                     DeliveryStrategy     = "managed"
	DeliveryUseInPlace                  DeliveryStrategy     = "use-in-place"
	DeliveryDestinationPull             DeliveryStrategy     = "destination-pull"
	DeliveryGatewayTransfer             DeliveryStrategy     = "gateway-transfer"
	DeliveryBuildAndTransfer            DeliveryStrategy     = "build-and-transfer"
	DeliveryPreferInPlace               DeliveryStrategy     = "prefer-in-place"
	ExecutableFormatOCI                 ExecutableFormat     = "oci"
	ExecutableFormatSIF                 ExecutableFormat     = "sif"
)

// ExecutableReference identifies software independently from its eventual
// local path. The planner resolves it into a ResolvedExecutable before run.
type ExecutableReference struct {
	Source   ExecutableSource   `json:"source"`
	Delivery ExecutableDelivery `json:"delivery"`
	// Deprecated internal compatibility fields removed from authored contract.
	/*
		SourceType      ExecutableSourceType `json:"-"`
		Reference       string               `json:"reference,omitempty"`
		ArtifactID      string               `json:"artifactId,omitempty"`
		Version         string               `json:"version,omitempty"`
		Path            string               `json:"path,omitempty"`
		EnvironmentID   string               `json:"environmentId,omitempty"`
		ResourceID      string               `json:"resourceId,omitempty"`
		Checksum        string               `json:"checksum,omitempty"`
		Format          ExecutableFormat     `json:"format,omitempty"`
		TargetFormat    ExecutableFormat     `json:"targetFormat,omitempty"`
		Delivery        DeliveryStrategy     `json:"delivery,omitempty"`
		BuildContext    string               `json:"buildContext,omitempty"`
		BuildRecipePath string               `json:"buildRecipePath,omitempty"`
	*/
}
type ExecutableSource struct {
	Type           ExecutableSourceType `json:"type"`
	Reference      string               `json:"reference,omitempty"`
	Path           string               `json:"path,omitempty"`
	URI            string               `json:"uri,omitempty"`
	ArtifactRef    *ArtifactReference   `json:"artifactRef,omitempty"`
	EnvironmentRef string               `json:"environmentRef,omitempty"`
	ResourceRef    string               `json:"resourceRef,omitempty"`
	ExpectedDigest string               `json:"expectedDigest,omitempty"`
	Format         ExecutableFormat     `json:"format,omitempty"`
	// ArtifactBuildRef identifies an immutable, server-created build
	// specification. Authored workflows never contain browser/host paths.
	ArtifactBuildRef string `json:"artifactBuildRef,omitempty"`
	CredentialRef    string `json:"credentialRef,omitempty"`
}

type ArtifactReference struct {
	ID      string `json:"id"`
	Version string `json:"version,omitempty"`
}

type ExecutableDelivery struct {
	Strategy     DeliveryStrategy `json:"strategy,omitempty"`
	TargetFormat ExecutableFormat `json:"targetFormat,omitempty"`
}

type ResolvedExecutable struct {
	VariantID           string           `json:"variantId,omitempty"`
	Digest              string           `json:"digest,omitempty"`
	Format              ExecutableFormat `json:"format"`
	LocalPath           string           `json:"localPath,omitempty"`
	RemotePath          string           `json:"remotePath,omitempty"`
	EnvironmentID       string           `json:"environmentId,omitempty"`
	ResourceID          string           `json:"resourceId,omitempty"`
	Delivery            DeliveryStrategy `json:"delivery"`
	MaterializationID   string           `json:"materializationId,omitempty"`
	MaterializationDone bool             `json:"materializationDone,omitempty"`
}

func (c ActivityCommand) EffectiveExecutable() *ExecutableReference {
	if c.Executable != nil {
		return c.Executable
	}
	if strings.TrimSpace(c.Image) == "" {
		return nil
	}
	return &ExecutableReference{Source: ExecutableSource{Type: ExecutableSourceOCI, Reference: c.Image}, Delivery: ExecutableDelivery{Strategy: DeliveryAuto}}
}

func (e ExecutableReference) Validate() error {
	if e.Source.Type == "" {
		return fmt.Errorf("executable source.type is required")
	}
	if e.Delivery.Strategy == "" {
		e.Delivery.Strategy = DeliveryAuto
	}
	switch e.Source.Type {
	case ExecutableSourceCatalog:
		if e.Source.ArtifactRef == nil || e.Source.ArtifactRef.ID == "" {
			return fmt.Errorf("catalog executable requires source.artifactRef")
		}
	case ExecutableSourceOCI, ExecutableSourceLocalContainerImage:
		if e.Source.Reference == "" {
			return fmt.Errorf("%s executable requires source.reference", e.Source.Type)
		}
	case ExecutableSourceBuild:
		if e.Source.ArtifactBuildRef == "" {
			return fmt.Errorf("build executable requires source.artifactBuildRef")
		}
	case ExecutableSourceLocalFile:
		if e.Source.Path == "" {
			return fmt.Errorf("local-file executable requires path")
		}
	case ExecutableSourceRemoteFile:
		if e.Source.Path == "" || (e.Source.EnvironmentRef == "" && e.Source.ResourceRef == "") {
			return fmt.Errorf("remote-file executable requires path and environmentRef or resourceRef")
		}
	case ExecutableSourceObjectStorage, ExecutableSourceHTTP:
		if e.Source.URI == "" {
			return fmt.Errorf("%s executable requires source.uri", e.Source.Type)
		}
	default:
		return fmt.Errorf("unsupported executable source.type %q", e.Source.Type)
	}
	return nil
}
