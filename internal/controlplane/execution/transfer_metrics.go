package execution

import "github.com/UFFeScience/akoflow/internal/domain"

// A verified internal filesystem copy has explicitly zero network traffic.
// Legacy observations lacking route evidence retain their previous byte count.
func observedNetworkBytes(transfer domain.DataTransferRun) int64 {
	if transfer.NetworkBytes > 0 {
		return transfer.NetworkBytes
	}
	if transfer.Route.SharedFilesystem != "" {
		return 0
	}
	return transfer.TransferredBytes
}
