package ssh_connection_service

import (
	"bytes"
	"fmt"
	"sync"

	ssh_client_entity "github.com/ovvesley/akoflow/pkg/client/entities/ssh_client"
	"golang.org/x/crypto/ssh"
)

type SSHConnectionService struct {
	hosts       []ssh_client_entity.SSHClient
	connections []*ssh.Client
}

func (s *SSHConnectionService) AddHost(host ssh_client_entity.SSHClient) {
	s.hosts = append(s.hosts, host)
}

func New() *SSHConnectionService {
	return &SSHConnectionService{}
}

func (s *SSHConnectionService) connect(client ssh_client_entity.SSHClient) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: client.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(client.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	address := fmt.Sprintf("%s:%d", client.Host, client.Port)
	connection, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %v", client.Host, err)
	}

	return connection, nil
}

func (s *SSHConnectionService) addConnection(connection *ssh.Client) {
	s.connections = append(s.connections, connection)
}

func (s *SSHConnectionService) EstablishConnectionWithHosts() {
	var wg sync.WaitGroup
	results := make(chan string, len(s.hosts))

	for _, client := range s.hosts {
		wg.Add(1)
		go func(sshClient ssh_client_entity.SSHClient) {
			defer wg.Done()
			s.establishConnection(sshClient, results)
		}(client)
	}

	wg.Wait()
	close(results)

	for result := range results {
		fmt.Println(result)
	}
}

func (s *SSHConnectionService) establishConnection(client ssh_client_entity.SSHClient, results chan<- string) {
	connection, err := s.connect(client)
	if err != nil {
		results <- fmt.Sprintf("Failed to connect to %s: %v", client.Host, err)
		return
	}

	results <- fmt.Sprintf("Connected to %s", client.Host)
	s.addConnection(connection)
}

func (s *SSHConnectionService) CloseConnections() {
	for _, connection := range s.connections {
		connection.Close()
	}
}

func (s *SSHConnectionService) ExecuteCommands(commands []string) {
	var wg sync.WaitGroup

	for _, client := range s.hosts {
		wg.Add(1)
		go func(sshClient ssh_client_entity.SSHClient) {
			defer wg.Done()
			s.executeCommandsOnHost(sshClient, commands)
		}(client)
	}

	wg.Wait()
}

func (s *SSHConnectionService) executeCommandsOnHost(client ssh_client_entity.SSHClient, commands []string) {
	connection, err := s.connect(client)
	if err != nil {
		fmt.Printf("Failed to connect to %s: %v\n", client.Host, err)
		return
	}
	defer connection.Close()

	for _, cmd := range commands {
		session, err := connection.NewSession()
		if err != nil {
			fmt.Printf("Failed to create session for command '%s' on host %s: %v\n", cmd, client.Host, err)
			return
		}
		defer session.Close()

		stdoutBuf := new(bytes.Buffer)
		session.Stdout = stdoutBuf

		err = session.Run(cmd)
		if err != nil {
			fmt.Printf("Failed to execute command '%s' on host %s: %v\n", cmd, client.Host, err)
			return
		}

		out := stdoutBuf.String()
		fmt.Printf("Output of command '%s' on host %s: %s\n", cmd, client.Host, out)
	}
}
