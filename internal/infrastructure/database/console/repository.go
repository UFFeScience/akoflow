package console

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	domainconsole "github.com/UFFeScience/akoflow/internal/domain/console"
)

type Repository struct{ db *sql.DB }

var _ ports.ConsoleCommandStore = (*Repository)(nil)
var _ ports.ConsoleSessionLogStore = (*Repository)(nil)

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) SaveConsoleSession(ctx context.Context, session domainconsole.Session) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO console_sessions
		(id,resource_id,runtime_id,connection_id,actor_id,status,external_id,failure,created_at,connected_at,finished_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET status=excluded.status,
		external_id=excluded.external_id,failure=excluded.failure,connected_at=excluded.connected_at,finished_at=excluded.finished_at`,
		session.ID, session.ResourceID, session.RuntimeID, session.ConnectionID, session.ActorID,
		session.Status, session.ExternalID, session.Failure, session.CreatedAt, session.ConnectedAt, session.FinishedAt)
	return err
}

func (r *Repository) SaveConsoleCommand(ctx context.Context, command domainconsole.Command) error {
	environment, err := json.Marshal(command.Environment)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO console_commands
		(id,resource_id,runtime_id,connection_id,actor_id,command_text,working_directory,environment,cpu_cores,
		memory_bytes,timeout_seconds,status,stdout,stderr,exit_code,external_id,failure,created_at,started_at,finished_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET
		status=excluded.status,stdout=excluded.stdout,stderr=excluded.stderr,exit_code=excluded.exit_code,
		external_id=excluded.external_id,failure=excluded.failure,finished_at=excluded.finished_at`,
		command.ID, command.ResourceID, command.RuntimeID, command.ConnectionID, command.ActorID, command.Command,
		command.WorkingDirectory, string(environment), command.CPUCores, command.MemoryBytes, command.TimeoutSeconds,
		command.Status, command.Stdout, command.Stderr, command.ExitCode, command.ExternalID, command.Failure,
		command.CreatedAt, command.StartedAt, command.FinishedAt)
	return err
}

const columns = `id,resource_id,runtime_id,connection_id,actor_id,command_text,working_directory,environment,
	cpu_cores,memory_bytes,timeout_seconds,status,stdout,stderr,exit_code,external_id,failure,created_at,started_at,finished_at`

func scan(scanner interface{ Scan(...any) error }) (*domainconsole.Command, error) {
	var command domainconsole.Command
	var environment string
	var exitCode sql.NullInt64
	var finishedAt sql.NullTime
	if err := scanner.Scan(&command.ID, &command.ResourceID, &command.RuntimeID, &command.ConnectionID, &command.ActorID,
		&command.Command, &command.WorkingDirectory, &environment, &command.CPUCores, &command.MemoryBytes, &command.TimeoutSeconds,
		&command.Status, &command.Stdout, &command.Stderr, &exitCode, &command.ExternalID, &command.Failure, &command.CreatedAt,
		&command.StartedAt, &finishedAt); err != nil {
		return nil, err
	}
	if environment != "" {
		if err := json.Unmarshal([]byte(environment), &command.Environment); err != nil {
			return nil, err
		}
	}
	if exitCode.Valid {
		value := int(exitCode.Int64)
		command.ExitCode = &value
	}
	if finishedAt.Valid {
		command.FinishedAt = &finishedAt.Time
	}
	return &command, nil
}

func (r *Repository) FindConsoleCommand(ctx context.Context, id string) (*domainconsole.Command, error) {
	command, err := scan(r.db.QueryRowContext(ctx, `SELECT `+columns+` FROM console_commands WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return command, err
}

func (r *Repository) ListConsoleCommands(ctx context.Context, limit int) ([]domainconsole.Command, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+columns+` FROM console_commands ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domainconsole.Command{}
	for rows.Next() {
		command, err := scan(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *command)
	}
	return items, rows.Err()
}

func (r *Repository) AppendConsoleSessionLog(ctx context.Context, sessionID, direction string, payload []byte, occurredAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO console_session_logs(session_id,direction,payload,occurred_at) VALUES (?,?,?,?)`, sessionID, direction, payload, occurredAt)
	return err
}

func (r *Repository) ReadConsoleSessionLog(ctx context.Context, sessionID string) ([]byte, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT payload FROM console_session_logs WHERE session_id=? ORDER BY id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var transcript []byte
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		transcript = append(transcript, payload...)
	}
	return transcript, rows.Err()
}
