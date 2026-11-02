package execution

import (
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestObservedNetworkBytesSeparatesSharedCopyFromTraffic(t *testing.T) {
	for _, test := range []struct {
		name string
		run  domain.DataTransferRun
		want int64
	}{
		{"shared Lustre", domain.DataTransferRun{TransferredBytes: 4096, Route: domain.TransferRoute{SharedFilesystem: "lustre"}}, 0},
		{"network rsync", domain.DataTransferRun{TransferredBytes: 4096, NetworkBytes: 4200}, 4200},
		{"legacy observation", domain.DataTransferRun{TransferredBytes: 4096}, 4096},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := observedNetworkBytes(test.run); got != test.want {
				t.Fatalf("network bytes=%d want=%d", got, test.want)
			}
		})
	}
}
