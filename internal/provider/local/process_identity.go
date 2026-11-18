package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func nativeProcessIdentity(pid int) (identity, state string, err error) {
	boot, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", "", err
	}
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return "", "", err
	}
	end := strings.LastIndexByte(string(stat), ')')
	if end < 0 {
		return "", "", fmt.Errorf("invalid local process state")
	}
	fields := strings.Fields(string(stat[end+1:]))
	if len(fields) <= 19 || strings.TrimSpace(string(boot)) == "" {
		return "", "", fmt.Errorf("incomplete local process identity")
	}
	return strings.TrimSpace(string(boot)) + ":" + fields[19], fields[0], nil
}

func NativeReservationReleased(processID, expected string) bool {
	pid, err := strconv.Atoi(processID)
	if err != nil || pid <= 0 || expected == "" {
		return false
	}
	current, state, err := nativeProcessIdentity(pid)
	if err != nil {
		var missing *os.PathError
		return errors.Is(err, os.ErrNotExist) && errors.As(err, &missing) && missing.Path == "/proc/"+processID+"/stat"
	}
	return current != expected || state == "Z" || state == "X"
}

func waitForNativeExit(ctx context.Context, processID, identity string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for !NativeReservationReleased(processID, identity) {
		select {
		case <-ctx.Done():
			return fmt.Errorf("local process %s has not stopped: %w", processID, ctx.Err())
		case <-ticker.C:
		}
	}
	return nil
}
