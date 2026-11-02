package mcpbridge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type requestIdentity struct{ principal, client string }
type identityContextKey struct{}

func requestContext(ctx context.Context, request *mcp.CallToolRequest) context.Context {
	identity := requestIdentity{principal: "shared", client: "mcp"}
	if request.Session != nil {
		if params := request.Session.InitializeParams(); params != nil && params.ClientInfo != nil {
			identity.client = safeClientName(params.ClientInfo.Name)
		}
	}
	if request.Extra != nil && request.Extra.TokenInfo != nil {
		identity.principal = request.Extra.TokenInfo.UserID
	}
	return context.WithValue(ctx, identityContextKey{}, identity)
}

func identityFromContext(ctx context.Context) requestIdentity {
	if identity, ok := ctx.Value(identityContextKey{}).(requestIdentity); ok {
		return identity
	}
	return requestIdentity{principal: "shared", client: "mcp"}
}

func safeClientName(value string) string {
	value = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._-", r) {
			return r
		}
		return '-'
	}, value)
	if len(value) > 64 {
		value = value[:64]
	}
	return value
}

func receiptID(ctx context.Context, id string) string {
	digest := sha256.Sum256([]byte(identityFromContext(ctx).principal + "\n" + id))
	return hex.EncodeToString(digest[:])
}

func (b *Bridge) ownsTransfer(id string, request *mcp.CallToolRequest) bool {
	return b.transfers.ownedBy(id, identityFromContext(requestContext(context.Background(), request)).principal)
}

func (s *transferStore) setOwner(id, owner string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if item := s.items[id]; item != nil {
		item.owner = owner
	}
}

func (s *transferStore) ownedBy(id, owner string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanup()
	item := s.items[id]
	if item == nil {
		return false
	}
	actual := item.owner
	if actual == "" {
		actual = "shared"
	}
	return actual == owner
}
