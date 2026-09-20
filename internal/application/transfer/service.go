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
	Connectors        []ports.TransferConnector
	Resolver          EndpointResolver
	Strategies        StrategyResolver
	Progress          TransferProgressStore
	ChunkSize         func(context.Context) int64
	VerifiedArtifacts *VerifiedArtifactCache
}

const defaultTransferChunkBytes int64 = 512 << 20

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
	run.Route = routeWithEndpoints(run.Route, plan)
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
	route = routeWithEndpoints(route, plan)
	// Destination pull requires an agent on the destination.
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
	m.operationEvent(ctx, plan, "started", "info", "Artifact materialization started", 0, blobsTotalBytes(plan.Blobs), map[string]any{
		"strategy": string(run.Strategy), "startedAt": run.StartedAt,
	})
	nextChunkIndex := 0
	for _, blob := range plan.Blobs {
		run.LogicalBytes += blob.SizeBytes
		finalName := blob.Digest
		if blob.Path != "" {
			finalName = blob.Path
		}
		final := destinationName(plan.Destination.Path, finalName)
		sizeBytes := blob.SizeBytes
		usedExisting := false
		m.operationEvent(ctx, plan, "queued", "info", "Artifact transfer queued", 0, sizeBytes,
			map[string]any{"artifactDigest": blob.Digest, "destination": final})
		shared, materializeErr := m.VerifiedArtifacts.Do(ctx, destination, final, blob.Digest, func() error {
			// Re-check after taking ownership: another request may have committed
			// the same immutable object while this caller was waiting.
			if ok, verifyErr := m.verify(ctx, dc, destination, final, blob.Digest); verifyErr == nil && ok {
				usedExisting = true
				return nil
			}
			m.operationEvent(ctx, plan, "started", "info", "Artifact upload started", 0, sizeBytes,
				map[string]any{"artifactDigest": blob.Digest, "destination": final, "startedAt": unixNow()})
			partial := final + ".partial"
			offset, sizeErr := m.size(ctx, dc, destination, partial)
			if sizeErr != nil {
				return sizeErr
			}
			sourceName := sourceName(plan.Source.Path, blob, len(plan.Blobs))
			if sizeBytes <= 0 {
				sizeBytes, sizeErr = m.size(ctx, sc, source, sourceName)
				if sizeErr != nil {
					return sizeErr
				}
			}
			routed, routeErr := m.transferBlob(ctx, plan, sc, dc, source, destination, sourceName,
				partial, blob, offset, sizeBytes, nextChunkIndex, chunkRuns, strategy, route, &run)
			if routeErr != nil {
				return routeErr
			}
			if routed && sizeBytes > offset {
				run.TransferredBytes += sizeBytes - offset
			}
			ok, verifyErr := m.verify(ctx, dc, destination, partial, blob.Digest)
			if verifyErr != nil {
				return verifyErr
			}
			if !ok {
				return fmt.Errorf("checksum mismatch for %s", blob.Digest)
			}
			return dc.Commit(ctx, destination, partial, final)
		})
		if materializeErr != nil {
			m.operationEvent(ctx, plan, "failed", "error", "Artifact transfer failed", run.TransferredBytes, sizeBytes,
				map[string]any{"artifactDigest": blob.Digest, "error": materializeErr.Error()})
			return failed(target, run, materializeErr)
		}
		if shared || usedExisting {
			run.Strategy = domain.TransferUseExisting
			run.Route.Strategy = domain.TransferUseExisting
			if shared {
				run.Route.Reason = "waited for in-flight artifact transfer; verified cache hit"
				m.operationEvent(ctx, plan, "completed", "info", "Waited for in-flight transfer and reused the verified artifact", sizeBytes, sizeBytes,
					map[string]any{"artifactDigest": blob.Digest, "destination": final, "cacheHit": true, "waitedForInFlight": true})
			} else {
				run.Route.Reason = "destination file already exists; checksum verified and cached"
				m.operationEvent(ctx, plan, "completed", "info", "Destination artifact already exists; cache hit", sizeBytes, sizeBytes,
					map[string]any{"artifactDigest": blob.Digest, "destination": final, "cacheHit": true})
			}
		}
		nextChunkIndex += chunkCount(sizeBytes, m.chunkSize(ctx))
		run.VerifiedBlobs = append(run.VerifiedBlobs, blob.Digest)
	}
	run.Status, run.FinishedAt = domain.TransferCompleted, unixNow()
	if err := m.saveProgress(ctx, run); err != nil {
		return failed(target, run, err)
	}
	m.operationEvent(ctx, plan, "completed", "info", "Artifact materialization completed", run.TransferredBytes, run.LogicalBytes,
		map[string]any{"strategy": string(run.Strategy), "networkBytes": run.NetworkBytes})
	target.Status = domain.MaterializationCommitted
	target.VerifiedDigest = target.Digest
	return target, run, nil
}

func blobsTotalBytes(blobs []domain.BlobDescriptor) int64 {
	var total int64
	for _, blob := range blobs {
		total += blob.SizeBytes
	}
	return total
}

func routeWithEndpoints(route domain.TransferRoute, plan domain.DataTransferPlan) domain.TransferRoute {
	if route.SourceResourceID == "" {
		route.SourceResourceID = plan.Source.ResourceID
	}
	if route.TargetResourceID == "" {
		route.TargetResourceID = plan.Destination.ResourceID
	}
	if route.SourceEnvironmentID == "" {
		route.SourceEnvironmentID = plan.Source.EnvironmentID
	}
	if route.TargetEnvironmentID == "" {
		route.TargetEnvironmentID = plan.Destination.EnvironmentID
	}
	return route
}

func (m Materializer) chunkSize(ctx context.Context) int64 {
	if m.ChunkSize != nil {
		if size := m.ChunkSize(ctx); size > 0 {
			return size
		}
	}
	return defaultTransferChunkBytes
}

func (m Materializer) transferBlob(
	ctx context.Context,
	plan domain.DataTransferPlan,
	sourceConnector ports.TransferConnector,
	destinationConnector ports.TransferConnector,
	source domain.TransferEndpoint,
	destination domain.TransferEndpoint,
	sourceName string,
	partial string,
	blob domain.BlobDescriptor,
	offset int64,
	sizeBytes int64,
	baseIndex int,
	persisted map[int]domain.TransferChunkRun,
	strategy domain.TransferStrategy,
	route domain.TransferRoute,
	run *domain.DataTransferRun,
) (bool, error) {
	if strategy == domain.TransferRuntimeLocal || strategy == domain.TransferDirectRuntime || strategy == domain.TransferSharedStorage {
		if routeConnector, ok := sourceConnector.(ports.TransferRouteConnector); ok {
			networkBytes, err := routeConnector.TransferRoute(ctx, strategy, source, destination, sourceName, partial, offset)
			if err == nil {
				if networkBytes < 0 && sizeBytes > offset {
					networkBytes = sizeBytes - offset
				}
				run.NetworkBytes += networkBytes
				return true, nil
			}
			if route.Fallback != domain.TransferGateway {
				return false, err
			}
			run.Strategy = domain.TransferGateway
			run.Route.Strategy = domain.TransferGateway
			run.Route.Reason = fmt.Sprintf("%s; direct operation failed (%v), used bounded Akoflow relay", route.Reason, err)
		} else if route.Fallback == domain.TransferGateway {
			run.Strategy = domain.TransferGateway
			run.Route.Strategy = domain.TransferGateway
			run.Route.Reason = route.Reason + "; connector has no runtime route support, used bounded Akoflow relay"
		}
	}
	if err := m.copyGatewayChunks(ctx, plan, sourceConnector, destinationConnector, source, destination,
		sourceName, partial, blob, offset, sizeBytes, baseIndex, persisted, run); err != nil {
		return false, err
	}
	return false, nil
}

func chunkCount(size, chunkSize int64) int {
	if size <= 0 || chunkSize <= 0 {
		return 0
	}
	return int((size + chunkSize - 1) / chunkSize)
}

func (m Materializer) copyGatewayChunks(ctx context.Context, plan domain.DataTransferPlan, sourceConnector, destinationConnector ports.TransferConnector, source, destination domain.TransferEndpoint, sourceName, partial string, blob domain.BlobDescriptor, offset, sizeBytes int64, baseIndex int, persisted map[int]domain.TransferChunkRun, run *domain.DataTransferRun) error {
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
		startedAt := unixNow()
		progress := &transferProgressReader{reader: io.TeeReader(limited, hash), total: sizeBytes,
			lastReported: time.Now(), report: func(transferred int64) {
				m.operationEvent(ctx, plan, "progress", "info", "Artifact upload in progress", current+transferred, sizeBytes,
					map[string]any{"artifactDigest": blob.Digest, "startedAt": startedAt})
			}}
		if err := destinationConnector.Put(ctx, destination, partial, progress, current); err != nil {
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
