package library

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/planning/algorithms"
)

type Package struct {
	Defaults       map[string]any    `json:"defaults,omitempty"`
	SchemaVersion  string            `json:"schemaVersion"`
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	Objective      string            `json:"objective"`
	Multiple       bool              `json:"multiple"`
	Origin         string            `json:"origin"`
	Exclusive      bool              `json:"exclusive"`
	Source         string            `json:"source,omitempty"`
	Files          map[string]string `json:"files,omitempty"`
	Documentation  string            `json:"documentation"`
	Revision       int               `json:"revision"`
	ActiveRevision int               `json:"activeRevision"`
	Status         string            `json:"status"`
	CompileError   string            `json:"compileError,omitempty"`
	SourceHash     string            `json:"sourceHash,omitempty"`
	UpdatedAt      string            `json:"updatedAt,omitempty"`
}

type Catalog struct {
	db       *sql.DB
	registry *algorithms.Registry
	root     string
	mu       sync.Mutex
	readOnly bool
}

func New(ctx context.Context, db *sql.DB, registry *algorithms.Registry, root string, readOnly bool) (*Catalog, error) {
	s := &Catalog{db: db, registry: registry, root: root, readOnly: readOnly}
	rows, err := db.QueryContext(ctx, `SELECT active_payload FROM scheduler_plugin_sources WHERE active_payload <> ''`)
	if err != nil {
		if readOnly {
			return s, nil
		}
		return nil, err
	}
	var active []Package
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return nil, err
		}
		var p Package
		if err = json.Unmarshal([]byte(raw), &p); err != nil {
			rows.Close()
			return nil, err
		}
		active = append(active, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, p := range active {
		binary, pathErr := filepath.Abs(s.binary(p))
		if pathErr != nil {
			return nil, pathErr
		}
		if _, err := os.Stat(binary); err != nil && !readOnly {
			binary, err = s.compile(ctx, p)
			if err != nil {
				current, getErr := s.Get(ctx, p.ID)
				if getErr == nil {
					current.Status = "unavailable"
					current.CompileError = err.Error()
					if persistErr := s.persist(ctx, *current, false); persistErr != nil {
						return nil, persistErr
					}
				}
				continue
			}
		}
		if _, err := os.Stat(binary); err == nil {
			s.registry.SetPlugin(&executable{descriptor: descriptor(p), binary: binary, revision: p.ActiveRevision, sourceHash: p.SourceHash})
		}
	}
	return s, nil
}
func descriptor(p Package) ports.SchedulerDescriptor {
	return ports.SchedulerDescriptor{ID: p.ID, Name: p.Name, Description: p.Description, Objective: p.Objective, Multiple: p.Multiple, Defaults: p.Defaults, Origin: "plugin"}
}
func native(id string) bool { return id == "heft" || id == "prism-time" || id == "prism-cost" }
func (s *Catalog) List(ctx context.Context) ([]Package, error) {
	out := []Package{}
	for _, d := range s.registry.List() {
		if !native(d.ID) {
			continue
		}
		out = append(out,
			Package{SchemaVersion: "akoflow.scheduler/v1",
				ID:             d.ID,
				Name:           d.Name,
				Description:    d.Description,
				Objective:      d.Objective,
				Multiple:       d.Multiple,
				Origin:         "native",
				Exclusive:      d.ID != "heft",
				Status:         "ready",
				Revision:       1,
				ActiveRevision: 1,
				Documentation:  nativeDocs(d.ID)})
	}
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM scheduler_plugin_sources ORDER BY id`)
	if err != nil {
		if s.readOnly {
			return out, nil
		}
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var p Package
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			return nil, err
		}
		p.Source = ""
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, rows.Err()
}
func (s *Catalog) Get(ctx context.Context, id string) (*Package, error) {
	if native(id) {
		list, err := s.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range list {
			if p.ID == id {
				p.Files = algorithms.Sources()
				return &p, nil
			}
		}
	}
	var raw string
	if err := s.db.QueryRowContext(ctx, `SELECT payload FROM scheduler_plugin_sources WHERE id=?`, id).Scan(&raw); err != nil {
		return nil, err
	}
	var p Package
	err := json.Unmarshal([]byte(raw), &p)
	return &p, err
}

var identifier = regexp.MustCompile(`^[a-z][a-z0-9-]{1,63}$`)

func (s *Catalog) Save(ctx context.Context, p Package) (*Package, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.readOnly {
		return nil, fmt.Errorf("instance is read-only")
	}
	if native(p.ID) {
		return nil, fmt.Errorf("native algorithms cannot be overwritten")
	}
	if p.SchemaVersion != "" && p.SchemaVersion != "akoflow.scheduler/v1" {
		return nil, fmt.Errorf("unsupported package schema")
	}
	if !identifier.MatchString(p.ID) || p.Name == "" || len(p.Source) == 0 || len(p.Source) > 512*1024 {
		return nil, fmt.Errorf("id, name and Go source (up to 512 KiB) are required")
	}
	if p.Objective != "time" && p.Objective != "cost" && p.Objective != "pareto" {
		return nil, fmt.Errorf("objective must be time, cost or pareto")
	}
	old, err := s.Get(ctx, p.ID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	p.Revision = 1
	p.ActiveRevision = 0
	if old != nil {
		p.Revision = old.Revision + 1
		p.ActiveRevision = old.ActiveRevision
	}
	p.SchemaVersion = "akoflow.scheduler/v1"
	p.Origin = "plugin"
	p.Exclusive = false
	p.Files = nil
	p.CompileError = ""
	p.Status = "compiling"
	p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	hash := sha256.Sum256([]byte(Contract + runner + p.Source))
	p.SourceHash = hex.EncodeToString(hash[:])
	if err = s.persist(ctx, p, false); err != nil {
		return nil, err
	}
	binary, compileErr := s.compile(ctx, p)
	if compileErr != nil {
		p.Status = "compile_failed"
		p.CompileError = compileErr.Error()
		if err = s.persist(context.WithoutCancel(ctx), p, false); err != nil {
			return nil, err
		}
		return &p, nil
	}
	p.Status = "ready"
	p.ActiveRevision = p.Revision
	if err = s.persist(ctx, p, true); err != nil {
		return nil, err
	}
	s.registry.SetPlugin(&executable{descriptor: descriptor(p), binary: binary, revision: p.ActiveRevision, sourceHash: p.SourceHash})
	return &p, nil
}
func (s *Catalog) persist(ctx context.Context, p Package, activate bool) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	active := ""
	if activate {
		active = string(raw)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_,
		err = tx.ExecContext(ctx,
		`INSERT INTO scheduler_plugin_sources(id,payload,active_payload) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET payload=excl`+
			`uded.payload,active_payload=CASE WHEN excluded.active_payload='' THEN scheduler_plugin_sources.active_payload ELSE exclu`+
			`ded.active_payload END`,
		p.ID,
		string(raw),
		active)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO scheduler_plugin_revisions(id,revision,payload) VALUES(?,?,?) ON CONFLICT(id,revision) DO UPDATE SET payload=excluded.payload`, p.ID, p.Revision, string(raw))
	if err != nil {
		return err
	}
	err = tx.Commit()
	return err
}
func (s *Catalog) binary(p Package) string {
	return filepath.Join(s.root, p.ID, p.SourceHash, "scheduler")
}
func nativeDocs(id string) string {
	if id == "heft" {
		return "HEFT ranks the workflow DAG by upward rank, then places each activity at its earliest feasible finish time across heterogeneous re" +
			"sources and CPU cores. It produces one baseline plan. It considers computation and data transfer estimates. Unlike PRISM, it does " +
			"not use the session interference matrix during planning. The returned plan is validated against the same frozen workflow and resou" +
			"rce scope."
	}
	if id == "prism-cost" {
		return "PRISM Cost is an Ak\u00f4Flow-exclusive PRISM-CC scheduler. It explores alternative placements with beam search and ranks candidate pla" +
			"ns exclusively by predicted financial cost. It models heterogeneous compute, shared network transfers, cloud lifecycle and optiona" +
			"l activity interference. optionCount controls retained plans; beamWidth controls the search frontier. The search is heuristic and " +
			"does not guarantee a global optimum."
	}
	return "PRISM Time is an Ak\u00f4Flow-exclusive PRISM-CC scheduler. It explores alternative placements with beam search and ranks candidate pla" +
		"ns exclusively by predicted makespan. It models heterogeneous compute, shared network transfers, cloud lifecycle and optional acti" +
		"vity interference. optionCount controls retained plans; beamWidth controls the search frontier. The search is heuristic and does n" +
		"ot guarantee a global optimum."
}
