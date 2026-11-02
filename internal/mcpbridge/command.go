package mcpbridge

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RunCommand implements both akoflow mcp and akoflow-server mcp without starting
// another daemon. Tokens are inherited from the environment, never CLI flags.
func RunCommand(ctx context.Context, args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("mcp", flag.ContinueOnError)
	flags.SetOutput(stderr)
	api := flags.String("api-url", envDefault("AKOFLOW_API_URL", "http://127.0.0.1:8080/akoflow-api"), "daemon API URL including /akoflow-api")
	transport := flags.String("transport", "stdio", "stdio or http")
	listen := flags.String("listen", "127.0.0.1:8081", "HTTP listen address")
	permissions := flags.String("permissions", envDefault("AKOFLOW_MCP_PERMISSIONS", "read"), "read, write or admin")
	directory := flags.String("state-directory", os.Getenv("AKOFLOW_MCP_STATE_DIRECTORY"), "private durable receipts and temporary transfer directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if *transport != "stdio" && *transport != "http" {
		return errors.New("transport must be stdio or http")
	}
	bridge, err := New(Options{APIURL: *api, APIToken: os.Getenv("AKOFLOW_API_TOKEN"), Permissions: Permission(*permissions), StateDirectory: *directory})
	if err != nil {
		return err
	}
	defer bridge.Close()
	if *transport == "stdio" {
		return bridge.server.Run(ctx, &mcp.StdioTransport{})
	}
	handler, err := bridge.HTTPHandler(HTTPOptionsFromEnvironment(os.Getenv("AKOFLOW_MCP_TOKEN"), splitOrigins(os.Getenv("AKOFLOW_MCP_ALLOWED_ORIGINS"))))
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	fmt.Fprintln(stderr, "AkôFlow MCP listening at", listener.Addr().String()+"/mcp")
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	mux.Handle("/.well-known/oauth-protected-resource", handler)
	mux.Handle("/.well-known/oauth-protected-resource/mcp", handler)
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: time.Minute}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(shutdown)
	}()
	if err = server.Serve(listener); errors.Is(err, http.ErrServerClosed) {
		<-shutdownDone
		return nil
	}
	return err
}

func envDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func splitOrigins(value string) []string {
	out := []string{}
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
