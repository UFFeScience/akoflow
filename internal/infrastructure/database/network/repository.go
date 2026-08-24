package network

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type Repository struct{ db *sql.DB }

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Create(ctx context.Context, topology domain.NetworkTopology) error {
	if err := validate(topology); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	metadata, err := json.Marshal(topology.Metadata)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO network_topologies
		(id, name, version, execution_scope_id, metadata) VALUES (?, ?, ?, ?, ?)`, topology.ID,
		topology.Name, topology.Version, topology.ExecutionScopeID, metadata); err != nil {
		return err
	}
	for _, link := range topology.Links {
		link.TopologyID = topology.ID
		linkMetadata, err := json.Marshal(link.Metadata)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO network_links (
			id, topology_id, source_resource_id, target_resource_id,
			bandwidth_bits_per_second, latency_seconds, price_per_byte,
			bidirectional, sharing_policy, max_concurrent_transfers, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, link.ID, topology.ID,
			link.SourceResourceID, link.TargetResourceID, link.BandwidthBitsPerSecond,
			link.LatencySeconds, link.PricePerByte, link.Bidirectional,
			link.SharingPolicy, link.MaxConcurrentTransfers, linkMetadata); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) Find(ctx context.Context, id string) (*domain.NetworkTopology, error) {
	var topology domain.NetworkTopology
	var metadata string
	err := r.db.QueryRowContext(ctx, `SELECT id, name, version, execution_scope_id, metadata
		FROM network_topologies WHERE id=?`, id).Scan(&topology.ID, &topology.Name,
		&topology.Version, &topology.ExecutionScopeID, &metadata)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(metadata), &topology.Metadata); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, topology_id, source_resource_id,
		target_resource_id, bandwidth_bits_per_second, latency_seconds,
		price_per_byte, bidirectional, sharing_policy, max_concurrent_transfers,
		metadata FROM network_links WHERE topology_id=? ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var link domain.NetworkLink
		var linkMetadata string
		if err := rows.Scan(&link.ID, &link.TopologyID, &link.SourceResourceID,
			&link.TargetResourceID, &link.BandwidthBitsPerSecond,
			&link.LatencySeconds, &link.PricePerByte, &link.Bidirectional,
			&link.SharingPolicy, &link.MaxConcurrentTransfers, &linkMetadata); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(linkMetadata), &link.Metadata); err != nil {
			return nil, err
		}
		topology.Links = append(topology.Links, link)
	}
	return &topology, rows.Err()
}

func (r *Repository) List(ctx context.Context) ([]domain.NetworkTopology, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM network_topologies ORDER BY name, version DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	topologies := make([]domain.NetworkTopology, 0, len(ids))
	for _, id := range ids {
		topology, err := r.Find(ctx, id)
		if err != nil {
			return nil, err
		}
		if topology != nil {
			topologies = append(topologies, *topology)
		}
	}
	return topologies, nil
}

func validate(topology domain.NetworkTopology) error {
	if topology.ID == "" || topology.Name == "" || topology.Version < 1 {
		return fmt.Errorf("network topology id, name and positive version are required")
	}
	if topology.ExecutionScopeID == "" {
		return fmt.Errorf("network topology execution scope is required")
	}
	seen := make(map[string]struct{}, len(topology.Links))
	for _, link := range topology.Links {
		if link.ID == "" || link.SourceResourceID == "" || link.TargetResourceID == "" || link.BandwidthBitsPerSecond <= 0 {
			return fmt.Errorf("network link id, endpoints and positive bandwidth are required")
		}
		if link.SourceResourceID == link.TargetResourceID {
			return fmt.Errorf("network link %q must connect different resources", link.ID)
		}
		if link.LatencySeconds < 0 || link.PricePerByte < 0 || link.MaxConcurrentTransfers < 0 {
			return fmt.Errorf("network link %q has negative latency, price or concurrency", link.ID)
		}
		if _, exists := seen[link.ID]; exists {
			return fmt.Errorf("duplicate network link %q", link.ID)
		}
		seen[link.ID] = struct{}{}
	}
	return nil
}
