package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func checkDependency(dep string) bool {
	cmd := exec.Command("which", dep)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func verifyDependencies() {
	deps := []string{"vpnc", "ssh", "sshpass"}
	missingDeps := []string{}

	for _, dep := range deps {
		if !checkDependency(dep) {
			missingDeps = append(missingDeps, dep)
		} else {
			fmt.Printf("✅ Dependência encontrada: %s\n", dep)
		}
	}

	if len(missingDeps) > 0 {
		fmt.Println("🚨 As seguintes dependências estão faltando:")
		for _, dep := range missingDeps {
			fmt.Printf("- %s\n", dep)
		}
		fmt.Println("💡 Por favor, instale as dependências manualmente antes de executar o script novamente.")
		os.Exit(1)
	}
}

func disconnectVPN() {
	fmt.Println("🔍 Verificando conexões VPN existentes...")
	disconnectCmd := exec.Command("sudo", "vpnc-disconnect")
	disconnectCmd.Stdout = os.Stdout
	disconnectCmd.Stderr = os.Stderr

	if err := disconnectCmd.Run(); err == nil {
		fmt.Println("🔌 Conexão VPN existente foi desconectada.")
	} else {
		fmt.Println("ℹ️ Nenhuma conexão VPN ativa encontrada ou erro ao desconectar.")
	}
}

func connectVPN(gateway, group, groupPassword, username, password string) {
	vpnConfig := fmt.Sprintf(`
IPSec gateway %s
IPSec ID %s
IPSec secret %s
Xauth username %s
Xauth password %s
`, gateway, group, groupPassword, username, password)

	configFile := fmt.Sprintf("/tmp/vpnc-config-%d.conf", time.Now().UnixNano())
	err := ioutil.WriteFile(configFile, []byte(vpnConfig), 0600)
	if err != nil {
		log.Fatalf("Erro ao criar arquivo de configuração VPN: %v", err)
	}

	connectCmd := exec.Command("sudo", "vpnc", configFile)
	connectCmd.Stdout = os.Stdout
	connectCmd.Stderr = os.Stderr
	fmt.Println("🔗 Conectando à VPN...")
	if err := connectCmd.Run(); err != nil {
		log.Fatalf("❌ Erro ao conectar à VPN: %v", err)
	}
	fmt.Println("✅ Conectado à VPN com sucesso!")
}

func waitForVPN(timeout int) bool {
	fmt.Println("⏳ Verificando se a VPN foi estabelecida...")

	for i := 0; i < timeout; i++ {
		cmd := exec.Command("ifconfig")
		output, err := cmd.Output()
		if err != nil {
			log.Printf("Erro ao verificar interfaces de rede: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		// Verifica por interfaces tun0, tun1, etc.
		if strings.Contains(string(output), "tun0") || strings.Contains(string(output), "tun1") {
			fmt.Println("✅ VPN está ativa e interface detectada.")
			return true
		}

		time.Sleep(1 * time.Second)
	}

	fmt.Println("❌ Timeout: VPN não foi estabelecida dentro do tempo limite.")
	return false
}

func checkIP() {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp4", addr)
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	resp, err := client.Get("https://ifconfig.me")
	if err != nil {
		log.Fatalf("Erro ao verificar o IP externo (IPv4): %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Erro ao ler resposta da requisição: %v", err)
	}

	fmt.Printf("🌐 IP externo atual (IPv4): %s\n", strings.TrimSpace(string(body)))
}

func connectAndListSSH(user, host, password string) {
	fmt.Printf("🔗 Conectando via SSH em %s@%s usando sshpass...\n", user, host)

	bashScript := `ls -la && cd /scratch_old/aidexl/wesley.ferreira/ && ls -la`

	// Comando SSH usando sshpass para passar a senha
	cmd := exec.Command(
		"sshpass", "-p", password,
		"ssh", "-o", "StrictHostKeyChecking=no", fmt.Sprintf("%s@%s", user, host),
		bashScript,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Executa o comando
	if err := cmd.Run(); err != nil {
		log.Fatalf("❌ Erro ao conectar via SSH ou listar arquivos: %v", err)
	}

	fmt.Println("✅ Listagem de arquivos concluída com sucesso usando sshpass!")
}

func main() {
	gateway := flag.String("gateway", "", "Endereço do gateway VPN")
	group := flag.String("group", "", "Nome do grupo VPN (ID)")
	groupPassword := flag.String("group-password", "", "Senha do grupo VPN (secret)")
	username := flag.String("username", "", "Nome de usuário para VPN")
	password := flag.String("password", "", "Senha do usuário para VPN")
	hostCluster := flag.String("host-cluster", "", "Host do Node de Login")

	flag.Parse()

	if *gateway == "" {
		*gateway = os.Getenv("VPN_GATEWAY")
	}
	if *group == "" {
		*group = os.Getenv("VPN_GROUP")
	}
	if *groupPassword == "" {
		*groupPassword = os.Getenv("VPN_GROUP_PASSWORD")
	}
	if *username == "" {
		*username = os.Getenv("VPN_USERNAME")
	}
	if *password == "" {
		*password = os.Getenv("VPN_PASSWORD")
	}

	if *hostCluster == "" {
		*hostCluster = os.Getenv("HOST_CLUSTER")
	}

	missingParams := []string{}
	if *gateway == "" {
		missingParams = append(missingParams, "gateway")
	}
	if *group == "" {
		missingParams = append(missingParams, "group")
	}
	if *groupPassword == "" {
		missingParams = append(missingParams, "group-password")
	}
	if *username == "" {
		missingParams = append(missingParams, "username")
	}
	if *password == "" {
		missingParams = append(missingParams, "password")
	}

	if *hostCluster == "" {
		missingParams = append(missingParams, "host-cluster")
	}

	if len(missingParams) > 0 {
		fmt.Printf("🚨 Os seguintes parâmetros estão faltando:\n")
		for _, param := range missingParams {
			fmt.Printf("- %s\n", param)
		}
		fmt.Println("💡 Use flags CLI ou defina as variáveis de ambiente correspondentes.")
		os.Exit(1)
	}

	verifyDependencies()
	// disconnectVPN()
	connectVPN(*gateway, *group, *groupPassword, *username, *password)

	if !waitForVPN(10) { // Aguarda até 10 segundos para a VPN se conectar
		log.Fatalf("❌ VPN não foi estabelecida.")
	}

	checkIP()

	// Conectar via SSH e listar arquivos
	connectAndListSSH(*username, *hostCluster, *password)

	// disconnectVPN()
}
