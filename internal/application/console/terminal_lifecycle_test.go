package console

import (
	"context"
	"io"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainconsole "github.com/UFFeScience/akoflow/internal/domain/console"
)

type terminalCloudStub struct{ released []string }

func (*terminalCloudStub) Provision(context.Context, string, domain.CloudProvisionRequest) (domain.CloudProvisionedInstance, error) {
	return domain.CloudProvisionedInstance{}, nil
}
func (*terminalCloudStub) Destroy(context.Context, string) (domain.CloudProvisionedInstance, error) {
	return domain.CloudProvisionedInstance{}, nil
}
func (*terminalCloudStub) Log(context.Context, string) ([]byte, error) { return nil, nil }
func (s *terminalCloudStub) Release(_ context.Context, targets []string) error {
	s.released = append(s.released, targets...)
	return nil
}

type lifecycleTerminalStub struct{ closed bool }

func (*lifecycleTerminalStub) Read([]byte) (int, error)        { return 0, io.EOF }
func (*lifecycleTerminalStub) Write(value []byte) (int, error) { return len(value), nil }
func (*lifecycleTerminalStub) Resize(uint16, uint16) error     { return nil }
func (s *lifecycleTerminalStub) Close() error                  { s.closed = true; return nil }

func TestClosingInteractiveCloudSessionReleasesCapacity(t *testing.T) {
	cloud := &terminalCloudStub{}
	terminal := &lifecycleTerminalStub{}
	controller := &TerminalController{cloud: cloud, sessions: map[string]*liveSession{}}
	session := &liveSession{
		value: domainconsole.Session{ID: "session"}, terminal: terminal,
		cloudTarget: "capacity-target", clients: map[*terminalClient]struct{}{},
	}
	controller.sessions[session.value.ID] = session
	controller.closeSession(session)
	if !terminal.closed || len(cloud.released) != 1 || cloud.released[0] != "capacity-target" {
		t.Fatalf("closed = %v, released = %#v", terminal.closed, cloud.released)
	}
}
