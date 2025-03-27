package ssh_client_entity

type SSHClient struct {
	Host         string
	Port         int
	Username     string
	Password     string
	IdentityFile string
}
