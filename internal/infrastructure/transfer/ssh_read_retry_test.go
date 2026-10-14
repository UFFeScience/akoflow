package transfer

import (
	"context"
	"errors"
	"testing"
)

func TestSSHReadRetryIsBoundedAndTransportOnly(t *testing.T) {
	for _, test := range []struct {
		message string
		calls   int
	}{
		{"Connection closed by UNKNOWN port 65535", 3},
		{"Permission denied (publickey)", 1},
		{"", 1},
	} {
		calls := 0
		_, err := retrySSHRead(context.Background(), 0, func() ([]byte, error) { calls++; return []byte(test.message), errors.New("failed") })
		if err == nil || calls != test.calls {
			t.Fatalf("%q calls=%d err=%v", test.message, calls, err)
		}
	}
}

func TestSSHReadRetryRecoversTransientDisconnect(t *testing.T) {
	calls := 0
	_, err := retrySSHRead(context.Background(), 0, func() ([]byte, error) {
		calls++
		if calls == 1 {
			return []byte("Connection reset"), errors.New("failed")
		}
		return nil, nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}
