package hubpublish

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func New(options Options) (*Manager, error) {
	if options.Directory == "" || options.Source == nil {
		return nil, errors.New("Hub connector needs private storage and a snapshot source")
	}
	manager := &Manager{options: options, previews: map[string]preparedPublication{}}
	manager.state.HubURL = options.HubURL
	data, err := os.ReadFile(filepath.Join(options.Directory, "connection.json"))
	if err == nil {
		if err = json.Unmarshal(data, &manager.state); err != nil {
			return nil, errors.New("saved Hub connection is invalid")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("cannot read saved Hub connection")
	}
	if err := manager.configureService(err == nil); err != nil {
		return nil, err
	}
	if err := manager.applyPendingReset(); err != nil {
		return nil, err
	}
	return manager, nil
}

func (m *Manager) persistLocked() error {
	if err := os.MkdirAll(m.options.Directory, 0700); err != nil {
		return errors.New("cannot create private Hub credential storage")
	}
	data, err := json.Marshal(m.state)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(m.options.Directory, ".connection-*")
	if err != nil {
		return errors.New("cannot save Hub connection")
	}
	name := file.Name()
	defer os.Remove(name)
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return errors.New("cannot securely save Hub connection")
	}
	if err = os.Rename(name, filepath.Join(m.options.Directory, "connection.json")); err != nil {
		return errors.New("cannot replace saved Hub connection")
	}
	return nil
}
