package ports

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainconsole "github.com/UFFeScience/akoflow/internal/domain/console"
)

type ConsoleCommandStore interface {
	SaveConsoleCommand(context.Context, domainconsole.Command) error
	ListConsoleCommands(context.Context, int) ([]domainconsole.Command, error)
	FindConsoleCommand(context.Context, string) (*domainconsole.Command, error)
}

type ConsoleCommandRunner interface {
	RunConsoleCommand(context.Context, domain.EnvironmentConnection, domain.Resource, domainconsole.Command) (stdout, stderr string, exitCode int, externalID string, err error)
}

type ConsoleCommands interface {
	ExecuteCommand(context.Context, domainconsole.Request) (domainconsole.Command, error)
	ListCommands(context.Context, int) ([]domainconsole.Command, error)
}

type InteractiveConsole interface {
	OpenSession(context.Context, domainconsole.SessionRequest) (domainconsole.Session, error)
	ListSessions(context.Context) ([]domainconsole.Session, error)
	CloseSession(context.Context, string) error
	SessionLog(context.Context, string) ([]byte, error)
	StreamSession(http.ResponseWriter, *http.Request, string)
}

// InteractiveTerminal is the bidirectional TTY attached to one remote session.
// It deliberately exposes a narrow interface so providers can implement SSH,
// local shells, or a future agent transport without leaking process details.
type InteractiveTerminal interface {
	io.Reader
	io.Writer
	Resize(rows, columns uint16) error
	Close() error
}

type InteractiveConsoleRunner interface {
	StartInteractive(context.Context, domain.EnvironmentConnection, domain.Resource) (InteractiveTerminal, error)
}

type ConsoleSessionLogStore interface {
	SaveConsoleSession(context.Context, domainconsole.Session) error
	AppendConsoleSessionLog(context.Context, string, string, []byte, time.Time) error
	ReadConsoleSessionLog(context.Context, string) ([]byte, error)
}
