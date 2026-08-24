package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/exec"
	"time"
)

type CommandExecutor interface {
	Run(context.Context, string, []string, []byte) ([]byte, error)
}

type OSCommandExecutor struct{}

func (OSCommandExecutor) Run(ctx context.Context, name string, args []string, input []byte) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdin = bytes.NewReader(input)
	output, err := command.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s: %w: %s", name, err, bytes.TrimSpace(output))
	}
	return output, nil
}

func NewID(prefix string) string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err == nil {
		return prefix + "-" + hex.EncodeToString(value)
	}
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func UnixSeconds(now time.Time) float64 {
	return float64(now.UnixNano()) / float64(time.Second)
}
