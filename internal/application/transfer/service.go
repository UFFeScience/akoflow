package transfer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"path"
	"sort"
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

type EndpointResolver interface {
	ResolveTransferEndpoint(context.Context, domain.TransferLocation) (domain.TransferEndpoint, error)
}

type TransferProgressStore interface {
	SaveTransferRun(context.Context, domain.DataTransferRun) error
	SaveTransferChunkRun(context.Context, domain.TransferChunkRun) error
	ListTransferChunkRuns(context.Context, string) ([]domain.TransferChunkRun, error)
}

type Materializer struct {
	Connectors []ports.TransferConnector
	Resolver   EndpointResolver
	Strategies StrategyResolver
	Progress   TransferProgressStore
	ChunkSize  func(context.Context) int64
}

func (m Materializer) endpoint(ctx context.Context, location domain.TransferLocation) (domain.TransferEndpoint, error) {
	if m.Resolver != nil {
		return m.Resolver.ResolveTransferEndpoint(ctx, location)
	}
	return domain.TransferEndpoint{URI: location.URI}, nil
}

func (m Materializer) connector(endpoint domain.TransferEndpoint) (ports.TransferConnector, error) {
	for _, c := range m.Connectors {
		if c.CanHandle(endpoint) {
			return c, nil
		}
	}
	return nil, fmt.Errorf("no connector for endpoint %q", endpoint.URI)
}
func (m Materializer) Materialize(ctx context.Context, plan domain.DataTransferPlan, target domain.ArtifactMaterialization) (domain.ArtifactMaterialization, domain.DataTransferRun, error) {
	run := domain.DataTransferRun{
		ID: plan.ID, PlanID: plan.ID, ExecutionRunID: plan.ExecutionRunID,
		ActivityID: plan.ConsumerActivityID, Strategy: plan.Strategy, Route: plan.Route,
		Status: domain.TransferPlanned,
	}
	source, err := m.endpoint(ctx, plan.Source)
	if err != nil {
		return failed(target, run, err)
	}
	destination, err := m.endpoint(ctx, plan.Destination)
	if err != nil {
		return failed(target, run, err)
	}
	sc, err := m.connector(source)
	if err != nil {
		return failed(target, run, err)
	}
	dc, err := m.connector(destination)
	if err != nil {
		return failed(target, run, err)
	}
	strategy := plan.Strategy
	route := plan.Route
	if strategy == "" {
		route = m.Strategies.Resolve(source, destination)
		strategy = route.Strategy
	} else if route.Strategy == "" {
		route = domain.TransferRoute{Strategy: strategy, Fallback: domain.TransferGateway, Reason: "strategy explicitly selected by the transfer plan"}
	}
	// This process is a gateway executor. A destination pull may only be run by
	// a destination agent; never silently turn a registry/HTTP reference into a
	// pull during Slurm submission.
	if strategy == domain.TransferDestinationPull {
		run.Strategy, run.Route = strategy, route
		return failed(target, run, fmt.Errorf("destination-pull requires a destination transfer agent"))
	}
	source = withTransferSession(source, plan.ID)
	destination = withTransferSession(destination, plan.ID)
	if err := beginTransferSession(ctx, sc, source); err != nil {
		run.Strategy, run.Route = strategy, route
		return failed(target, run, err)
	}
	defer endTransferSession(sc, source)
	if source.URI != destination.URI {
		if err := beginTransferSession(ctx, dc, destination); err != nil {
			run.Strategy, run.Route = strategy, route
			return failed(target, run, err)
		}
		defer endTransferSession(dc, destination)
	}
	run.Strategy, run.Route, run.Status, run.StartedAt = strategy, route, domain.TransferRunning, unixNow()
	chunkRuns, err := m.persistedChunks(ctx, run.ID)
	if err != nil {
		return failed(target, run, err)
	}
	for index, chunk := range chunkRuns {
		if chunk.Status == domain.TransferCompleted {
			run.CompletedChunks = append(run.CompletedChunks, index)
		}
	}
	sort.Ints(run.CompletedChunks)
	if err := m.saveProgress(ctx, run); err != nil {
		return failed(target, run, err)
	}
	nextChunkIndex := 0
	for _, blob := range plan.Blobs {
		run.LogicalBytes += blob.SizeBytes
		finalName := blob.Digest
		if blob.Path != "" {
			finalName = blob.Path
		}
		final := destinationName(plan.Destination.Path, finalName)
		// A complete matching object is an idempotent, no-copy materialization.
		if ok, verifyErr := m.verify(ctx, dc, destination, final, blob.Digest, blob.SizeBytes); verifyErr == nil && ok {
			run.VerifiedBlobs = append(run.VerifiedBlobs, blob.Digest)
			nextChunkIndex += chunkCount(blob.SizeBytes, m.chunkSize(ctx))
			continue
		}
		partial := final + ".partial"
		offset, err := m.size(ctx, dc, destination, partial)
		if err != nil {
			return failed(target, run, err)
		}
		sourceName := sourceName(plan.Source.Path, blob, len(plan.Blobs))
		// The transfer plan is content-addressed, so SizeBytes is the verified
		// object length. A resumed copy transfers only the remaining bytes.
		sizeBytes := blob.SizeBytes
		if sizeBytes <= 0 {
			sizeBytes, err = m.size(ctx, sc, source, sourceName)
			if err != nil {
				return failed(target, run, err)
			}
		}
		routed := false
		if strategy == domain.TransferRuntimeLocal || strategy == domain.TransferDirectRuntime || strategy == domain.TransferSharedStorage {
			if routeConnector, ok := sc.(ports.TransferRouteConnector); ok {
				networkBytes, routeErr := routeConnector.TransferRoute(ctx, strategy, source, destination, sourceName, partial, offset)
				if routeErr == nil {
					if networkBytes < 0 && sizeBytes > offset {
						networkBytes = sizeBytes - offset
					}
					run.NetworkBytes += networkBytes
					routed = true
				} else if route.Fallback != domain.TransferGateway {
					return failed(target, run, routeErr)
				} else {
					run.Strategy = domain.TransferGateway
					run.Route.Strategy = domain.TransferGateway
					run.Route.Reason = fmt.Sprintf("%s; direct operation failed (%v), used bounded Akoflow relay", route.Reason, routeErr)
				}
			} else if route.Fallback == domain.TransferGateway {
				run.Strategy = domain.TransferGateway
				run.Route.Strategy = domain.TransferGateway
				run.Route.Reason = route.Reason + "; connector has no runtime route support, used bounded Akoflow relay"
			}
		}
		if !routed {
			if err = m.copyGatewayChunks(ctx, sc, dc, source, destination, sourceName, partial, blob, offset, sizeBytes, nextChunkIndex, chunkRuns, &run); err != nil {
				return failed(target, run, err)
			}
		}
		nextChunkIndex += chunkCount(sizeBytes, m.chunkSize(ctx))
		if routed && sizeBytes > offset {
			run.TransferredBytes += sizeBytes - offset
		}
		ok, err := m.verify(ctx, dc, destination, partial, blob.Digest, sizeBytes)
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
	if err := m.saveProgress(ctx, run); err != nil {
		return failed(target, run, err)
	}
	target.Status = domain.MaterializationCommitted
	target.VerifiedDigest = target.Digest
	return target, run, nil
}

func (m Materializer) chunkSize(ctx context.Context) int64 {
	if m.ChunkSize != nil {
		if size := m.ChunkSize(ctx); size > 0 {
			return size
		}
	}
	return 8 << 20
}

func chunkCount(size, chunkSize int64) int {
	if size <= 0 || chunkSize <= 0 {
		return 0
	}
	return int((size + chunkSize - 1) / chunkSize)
}

func (m Materializer) copyGatewayChunks(ctx context.Context, sourceConnector, destinationConnector ports.TransferConnector, source, destination domain.TransferEndpoint, sourceName, partial string, blob domain.BlobDescriptor, offset, sizeBytes int64, baseIndex int, persisted map[int]domain.TransferChunkRun, run *domain.DataTransferRun) error {
	input, err := sourceConnector.Open(ctx, source, sourceName, offset)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			_ = input.Close()
		}
	}()
	chunkSize := m.chunkSize(ctx)
	current := offset
	for current < sizeBytes {
		size := chunkSize
		if remaining := sizeBytes - current; remaining < size {
			size = remaining
		}
		index := baseIndex + int(current/chunkSize)
		attempts := persisted[index].Attempts + 1
		chunk := domain.TransferChunkRun{TransferRunID: run.ID, Index: index, Offset: current, SizeBytes: size, Status: domain.TransferRunning, Attempts: attempts}
		if err := m.saveChunk(ctx, chunk); err != nil {
			return err
		}
		hash := sha256.New()
		limited := io.LimitReader(input, size)
		if err := destinationConnector.Put(ctx, destination, partial, io.TeeReader(limited, hash), current); err != nil {
			chunk.Status = domain.TransferFailed
			_ = m.saveChunk(ctx, chunk)
			return err
		}
		chunk.Digest = fmt.Sprintf("sha256:%x", hash.Sum(nil))
		chunk.Status = domain.TransferCompleted
		if err := m.saveChunk(ctx, chunk); err != nil {
			return err
		}
		current += size
		run.TransferredBytes += size
		run.NetworkBytes += size
		if !containsChunk(run.CompletedChunks, index) {
			run.CompletedChunks = append(run.CompletedChunks, index)
			sort.Ints(run.CompletedChunks)
		}
		if err := m.saveProgress(ctx, *run); err != nil {
			return err
		}
	}
	if err := input.Close(); err != nil {
		return fmt.Errorf("close source stream for %s: %w", blob.Digest, err)
	}
	closed = true
	return nil
}

func (m Materializer) persistedChunks(ctx context.Context, transferRunID string) (map[int]domain.TransferChunkRun, error) {
	result := map[int]domain.TransferChunkRun{}
	if m.Progress == nil {
		return result, nil
	}
	values, err := m.Progress.ListTransferChunkRuns(ctx, transferRunID)
	if err != nil {
		return nil, fmt.Errorf("load persisted chunks for %s: %w", transferRunID, err)
	}
	for _, value := range values {
		result[value.Index] = value
	}
	return result, nil
}

func containsChunk(values []int, expected int) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func (m Materializer) saveProgress(ctx context.Context, run domain.DataTransferRun) error {
	if m.Progress == nil {
		return nil
	}
	return m.Progress.SaveTransferRun(ctx, run)
}

func (m Materializer) saveChunk(ctx context.Context, chunk domain.TransferChunkRun) error {
	if m.Progress == nil {
		return nil
	}
	return m.Progress.SaveTransferChunkRun(ctx, chunk)
}

func withTransferSession(endpoint domain.TransferEndpoint, id string) domain.TransferEndpoint {
	configuration := make(map[string]string, len(endpoint.Configuration)+1)
	for key, value := range endpoint.Configuration {
		configuration[key] = value
	}
	configuration["transferSessionId"] = id
	endpoint.Configuration = configuration
	return endpoint
}

func beginTransferSession(ctx context.Context, connector ports.TransferConnector, endpoint domain.TransferEndpoint) error {
	if session, ok := connector.(ports.TransferSessionConnector); ok {
		return session.BeginTransferSession(ctx, endpoint)
	}
	return nil
}

func endTransferSession(connector ports.TransferConnector, endpoint domain.TransferEndpoint) {
	if session, ok := connector.(ports.TransferSessionConnector); ok {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = session.EndTransferSession(cleanupCtx, endpoint)
	}
}

func sourceName(configured string, blob domain.BlobDescriptor, count int) string {
	if configured != "" && count == 1 {
		return configured
	}
	if blob.Path != "" {
		return blob.Path
	}
	return blob.Digest
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
func (m Materializer) verify(ctx context.Context, c ports.TransferConnector, endpoint domain.TransferEndpoint, name, expected string, expectedSize int64) (bool, error) {
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
	size, err := io.Copy(hash, r)
	if err != nil {
		return false, err
	}
	if expectedSize > 0 && size != expectedSize {
		return false, nil
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
