package utils_parser_params_ssh_client

import (
	"strconv"
	"strings"

	ssh_client_entity "github.com/ovvesley/akoflow/pkg/client/entities/ssh_client"
)

type UtilsParserParamsSSHClient struct {
	IdentityFile string
}

func New() *UtilsParserParamsSSHClient {
	return &UtilsParserParamsSSHClient{}
}

func (u *UtilsParserParamsSSHClient) SetIdentityFile(identityFile string) *UtilsParserParamsSSHClient {
	u.IdentityFile = identityFile
	return u
}

func (u *UtilsParserParamsSSHClient) Parse(args string) []ssh_client_entity.SSHClient {
	var clients []ssh_client_entity.SSHClient
	entries := strings.Split(args, ",")
	for _, entry := range entries {
		parts := strings.Split(entry, "@")
		if len(parts) != 2 {
			continue
		}
		userInfo := strings.Split(parts[0], ":")
		if len(userInfo) != 2 {
			continue
		}
		hostInfo := strings.Split(parts[1], ":")
		if len(hostInfo) != 2 {
			continue
		}
		port, err := strconv.Atoi(hostInfo[1])
		if err != nil {
			continue
		}
		client := ssh_client_entity.SSHClient{
			Username:     userInfo[0],
			Password:     userInfo[1],
			Host:         hostInfo[0],
			Port:         port,
			IdentityFile: u.IdentityFile,
		}
		clients = append(clients, client)

	}
	return clients
}
