package resource

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNetworkLinkTransferSecondsConvertsBitsToBytes(t *testing.T) {
	link := NetworkLink{BandwidthBitsPerSecond: 8_000_000, LatencySeconds: 0.1}
	require.InDelta(t, 1.1, link.TransferSeconds(1_000_000), 1e-9)
}
