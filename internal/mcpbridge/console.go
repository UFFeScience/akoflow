package mcpbridge

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (b *Bridge) console(ctx context.Context, target *url.URL, args arguments) *mcp.CallToolResult {
	if len(args.Input) > 65536 {
		return failure("terminal input exceeds 64 KiB")
	}
	wait := args.WaitMillis
	if wait == 0 {
		wait = 1000
	}
	if wait < 1 || wait > 5000 {
		return failure("waitMillis must be between 1 and 5000")
	}
	wsURL := *target
	if target.Scheme == "https" {
		wsURL.Scheme = "wss"
	} else {
		wsURL.Scheme = "ws"
	}
	header := http.Header{}
	header.Set("Authorization", "Bearer "+b.options.APIToken)
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	if transport, ok := b.client.Transport.(*http.Transport); ok {
		dialer.TLSClientConfig = transport.TLSClientConfig
	}
	connection, response, err := dialer.DialContext(ctx, wsURL.String(), header)
	if err != nil {
		if response != nil {
			defer response.Body.Close()
			return b.response(ctx, response, "stream_console_session")
		}
		return failure("could not connect to daemon console")
	}
	defer connection.Close()
	connection.SetReadLimit(1 << 20)
	deadline := time.Now().Add(time.Duration(wait) * time.Millisecond)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	connection.SetWriteDeadline(deadline)
	if args.Input != "" {
		if err = connection.WriteMessage(websocket.BinaryMessage, []byte(args.Input)); err != nil {
			return failure("console input outcome is unknown; inspect session log before retrying")
		}
	}
	connection.SetReadDeadline(deadline)
	data := []byte{}
	truncated := false
	for {
		_, payload, readErr := connection.ReadMessage()
		if readErr != nil {
			var networkError net.Error
			normal := websocket.IsCloseError(readErr, websocket.CloseNormalClosure, websocket.CloseGoingAway)
			timedOut := errors.As(readErr, &networkError) && networkError.Timeout()
			if !normal && !timedOut {
				truncated = true
			}
			break
		}
		if len(data)+len(payload) > 1<<20 {
			truncated = true
			break
		}
		data = append(data, payload...)
	}
	return output(map[string]any{
		"data":      b.redactor.text(string(data)),
		"truncated": truncated,
		"notice":    "Output includes available session history. Use export_console_session_log for the complete persisted log. Read timeout does not imply command completion.",
	}, false)
}
