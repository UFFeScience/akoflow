package mcpbridge

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestOAuthSubjectsDoNotShareTransfersOrReceipts(t *testing.T) {
	var calls atomic.Int64
	bridge := testBridge(t, Write, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"created"}`))
	})
	id, _, err := bridge.transfers.save(strings.NewReader("private-input"))
	if err != nil {
		t.Fatal(err)
	}
	bridge.transfers.setOwner(id, "subject-a")
	request := func(subject string) *mcp.CallToolRequest {
		return &mcp.CallToolRequest{Extra: &mcp.RequestExtra{TokenInfo: &auth.TokenInfo{UserID: subject}}}
	}
	if !bridge.ownsTransfer(id, request("subject-a")) || bridge.ownsTransfer(id, request("subject-b")) {
		t.Fatal("transfer subject isolation failed")
	}
	raw := json.RawMessage(`{"requestId":"same-id","body":{}}`)
	op := findOperation(t, "create_workflow")
	for _, subject := range []string{"subject-a", "subject-a", "subject-b"} {
		ctx := requestContext(context.Background(), request(subject))
		if bridge.call(ctx, op, raw).IsError {
			t.Fatal("subject call failed")
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("expected one dispatch per subject, got %d", calls.Load())
	}
}
