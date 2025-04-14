package database_config

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"
)

type DatabaseConfig struct {
	Host     string
	HTTPPort string
	RaftPort string
	JoinURL  string
	Storage  string
}

func Load() DatabaseConfig {
	return DatabaseConfig{
		Host:     os.Getenv("DATABASE_HOST"),
		HTTPPort: os.Getenv("DATABASE_HTTP_PORT"),
		RaftPort: os.Getenv("DATABASE_RAFT_PORT"),
		JoinURL:  os.Getenv("DATABASE_JOIN_URL"),
		Storage:  "../../storage/",
	}
}

func IsPortAvailable(host, port string) bool {
	address := net.JoinHostPort(host, port)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return true
	}
	conn.Close()
	return false
}

func waitForDatabaseReady(cfg DatabaseConfig, maxRetries int, delay time.Duration) error {
	url := fmt.Sprintf("http://%s/status", net.JoinHostPort(cfg.Host, cfg.HTTPPort))

	for i := 0; i < maxRetries; i++ {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			var body map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&body); err == nil {
				if store, ok := body["store"].(map[string]any); ok {
					if state, ok := store["raft"].(map[string]any)["state"].(string); ok && state != "" {
						fmt.Printf("rqlited está pronto com estado: %s\n", state)
						return nil
					}
				}
			}
		}
		time.Sleep(delay)
	}
	return fmt.Errorf("rqlited não ficou pronto após %d tentativas", maxRetries)
}

func tryStartRqlite(cmd *exec.Cmd, retries int) error {
	for i := 0; i < retries; i++ {
		err := cmd.Start()
		if err == nil {
			return nil
		}
		fmt.Printf("Tentativa %d falhou ao iniciar rqlited: %v\n", i+1, err)
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("não foi possível iniciar o rqlited após %d tentativas", retries)
}

func StartRaftLeader(cfg DatabaseConfig) {
	if !IsPortAvailable(cfg.Host, cfg.HTTPPort) || !IsPortAvailable(cfg.Host, cfg.RaftPort) {
		fmt.Printf("rqlited já está rodando em %s:%s ou %s:%s — aguardando disponibilidade\n", cfg.Host, cfg.HTTPPort, cfg.Host, cfg.RaftPort)
		err := waitForDatabaseReady(cfg, 10, 1*time.Second)
		if err != nil {
			panic(err)
		}
		return
	}

	cmd := exec.Command("rqlited",
		"-http-addr", net.JoinHostPort(cfg.Host, cfg.HTTPPort),
		"-raft-addr", net.JoinHostPort(cfg.Host, cfg.RaftPort),
		cfg.Storage,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := tryStartRqlite(cmd, 3); err != nil {
		panic(err)
	}

	if err := waitForDatabaseReady(cfg, 10, 1*time.Second); err != nil {
		panic(err)
	}
}

func StartRaftFollower(cfg DatabaseConfig) {
	if !IsPortAvailable(cfg.Host, cfg.HTTPPort) || !IsPortAvailable(cfg.Host, cfg.RaftPort) {
		fmt.Printf("rqlited já está rodando em %s:%s ou %s:%s — aguardando disponibilidade\n", cfg.Host, cfg.HTTPPort, cfg.Host, cfg.RaftPort)
		err := waitForDatabaseReady(cfg, 10, 1*time.Second)
		if err != nil {
			panic(err)
		}
		return
	}

	cmd := exec.Command("rqlited",
		"-http-addr", net.JoinHostPort(cfg.Host, cfg.HTTPPort),
		"-raft-addr", net.JoinHostPort(cfg.Host, cfg.RaftPort),
		"-join", cfg.JoinURL,
		cfg.Storage,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := tryStartRqlite(cmd, 3); err != nil {
		panic(err)
	}

	if err := waitForDatabaseReady(cfg, 10, 1*time.Second); err != nil {
		panic(err)
	}
}
