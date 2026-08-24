package transfer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type Planner struct{}

func (Planner) Plan(source, destination domain.TransferLocation, blobs []domain.BlobDescriptor, available map[string]bool) domain.DataTransferPlan {
	plan := domain.DataTransferPlan{Source: source, Destination: destination, Strategy: domain.TransferSourcePush}
	if source.URI == destination.URI && source.Path == destination.Path {
		plan.Strategy = domain.TransferUseExisting
		return plan
	}
	for _, blob := range blobs {
		if !available[blob.Digest] {
			plan.Blobs = append(plan.Blobs, blob)
		}
	}
	return plan
}

type Materializer struct{ Connectors []ports.TransferConnector }

func (m Materializer) connector(endpoint domain.TransferEndpoint) (ports.TransferConnector, error) {
	for _, c := range m.Connectors {
		if c.CanHandle(endpoint) {
			return c, nil
		}
	}
	return nil, fmt.Errorf("no connector for endpoint %q", endpoint.URI)
}
func (m Materializer) Materialize(ctx context.Context, plan domain.DataTransferPlan, target domain.ArtifactMaterialization) (domain.ArtifactMaterialization, domain.DataTransferRun, error) {
	source := domain.TransferEndpoint{URI: plan.Source.URI}
	destination := domain.TransferEndpoint{URI: plan.Destination.URI}
	sc, err := m.connector(source)
	if err != nil {
		return target, domain.DataTransferRun{}, err
	}
	dc, err := m.connector(destination)
	if err != nil {
		return target, domain.DataTransferRun{}, err
	}
	strategy := plan.Strategy
	if strategy == "" {
		strategy = domain.TransferSourcePush
	}
	// This process is a gateway executor. A destination pull may only be run by
	// a destination agent; never silently turn a registry/HTTP reference into a
	// pull during Slurm submission.
	if strategy == domain.TransferDestinationPull {
		return failed(target, domain.DataTransferRun{ID: plan.ID, PlanID: plan.ID, Strategy: strategy, Status: domain.TransferPlanned}, fmt.Errorf("destination-pull requires a destination transfer agent"))
	}
	run := domain.DataTransferRun{ID: plan.ID, PlanID: plan.ID, Strategy: strategy,
		Status: domain.TransferRunning, StartedAt: unixNow()}
	for _, blob := range plan.Blobs {
		final := destinationName(plan.Destination.Path, blob.Digest)
		// A complete matching object is an idempotent, no-copy materialization.
		if ok, verifyErr := m.verify(ctx, dc, destination, final, blob.Digest); verifyErr == nil && ok {
			run.VerifiedBlobs = append(run.VerifiedBlobs, blob.Digest)
			continue
		}
		partial := final + ".partial"
		offset, err := m.size(ctx, dc, destination, partial)
		if err != nil {
			return failed(target, run, err)
		}
		sourceName := sourceName(plan.Source.Path, blob.Digest, len(plan.Blobs))
		input, err := sc.Open(ctx, source, sourceName, offset)
		if err != nil {
			return failed(target, run, err)
		}
		if err = dc.Put(ctx, destination, partial, input, offset); err != nil {
			input.Close()
			return failed(target, run, err)
		}
		input.Close()
		// The transfer plan is content-addressed, so SizeBytes is the verified
		// object length. A resumed copy transfers only the remaining bytes.
		sizeBytes := blob.SizeBytes
		if sizeBytes <= 0 {
			sizeBytes, err = m.size(ctx, sc, source, sourceName)
			if err != nil {
				return failed(target, run, err)
			}
		}
		if sizeBytes > offset {
			run.TransferredBytes += sizeBytes - offset
		}
		ok, err := m.verify(ctx, dc, destination, partial, blob.Digest)
		if err != nil || !ok {
			if err == nil {
				err = fmt.Errorf("checksum mismatch for %s", blob.Digest)
			}
			return failed(target, run, err)
		}
		if err = dc.Commit(ctx, destination, partial, final); err != nil {
			return failed(target, run, err)
		}
		run.VerifiedBlobs = append(run.VerifiedBlobs, blob.Digest)
	}
	run.Status, run.FinishedAt = domain.TransferCompleted, unixNow()
	target.Status = domain.MaterializationCommitted
	target.VerifiedDigest = target.Digest
	return target, run, nil
}

func sourceName(configured, digest string, count int) string {
	if configured != "" && count == 1 {
		return configured
	}
	return digest
}
func destinationName(root, digest string) string {
	if root == "" {
		return digest
	}
	return path.Join(root, digest)
}
func (m Materializer) size(ctx context.Context, c ports.TransferConnector, endpoint domain.TransferEndpoint, name string) (int64, error) {
	exists, err := c.Exists(ctx, endpoint, name)
	if err != nil || !exists {
		return 0, err
	}
	r, err := c.Open(ctx, endpoint, name, 0)
	if err != nil {
		return 0, err
	}
	defer r.Close()
	return io.Copy(io.Discard, r)
}
func (m Materializer) verify(ctx context.Context, c ports.TransferConnector, endpoint domain.TransferEndpoint, name, expected string) (bool, error) {
	exists, err := c.Exists(ctx, endpoint, name)
	if err != nil || !exists {
		return false, err
	}
	r, err := c.Open(ctx, endpoint, name, 0)
	if err != nil {
		return false, err
	}
	defer r.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, r); err != nil {
		return false, err
	}
	return strings.TrimPrefix(fmt.Sprintf("sha256:%x", hash.Sum(nil)), "sha256:") == strings.TrimPrefix(expected, "sha256:"), nil
}
func failed(target domain.ArtifactMaterialization, run domain.DataTransferRun, err error) (domain.ArtifactMaterialization, domain.DataTransferRun, error) {
	target.Status, run.Status, run.Error = domain.MaterializationFailed, domain.TransferFailed, err.Error()
	if run.FinishedAt == 0 {
		run.FinishedAt = unixNow()
	}
	return target, run, err
}

func unixNow() float64 { return float64(time.Now().UnixNano()) / float64(time.Second) }
