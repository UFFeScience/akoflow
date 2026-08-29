package transfer

import (
	"context"
	"io"
	"sync"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	domaininstance "github.com/UFFeScience/akoflow/internal/domain/instance"
)

type BufferSizeProvider func(context.Context) int

func InstanceBufferSize(store ports.InstanceStore) BufferSizeProvider {
	return func(ctx context.Context) int {
		value, err := store.Find(ctx)
		if err != nil || value == nil {
			return int(domaininstance.DefaultTransferBufferBytes)
		}
		return normalizeBufferSize(value.TransferBufferBytes)
	}
}

type relayChunk struct {
	data []byte
	err  error
}

// boundedRelay decouples a slow source connection from a destination stdin
// while retaining at most totalBuffer bytes. Small pooled chunks let the
// producer refill the queue while the consumer drains it.
type boundedRelay struct {
	chunks  chan relayChunk
	stop    chan struct{}
	current []byte
	err     error
	once    sync.Once
}

func newBoundedRelay(source io.Reader, totalBuffer int) *boundedRelay {
	if totalBuffer < 2 {
		totalBuffer = int(domaininstance.DefaultTransferBufferBytes)
	}
	chunkCount := 16
	chunkSize := totalBuffer / chunkCount
	if chunkSize < 4<<10 {
		chunkSize = 4 << 10
		chunkCount = totalBuffer / chunkSize
	}
	if chunkCount < 2 {
		chunkCount = 2
	}
	relay := &boundedRelay{chunks: make(chan relayChunk, chunkCount), stop: make(chan struct{})}
	go func() {
		defer close(relay.chunks)
		for {
			buffer := make([]byte, chunkSize)
			count, err := source.Read(buffer)
			if count > 0 {
				select {
				case relay.chunks <- relayChunk{data: buffer[:count]}:
				case <-relay.stop:
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					select {
					case relay.chunks <- relayChunk{err: err}:
					case <-relay.stop:
					}
				}
				return
			}
		}
	}()
	return relay
}

func (relay *boundedRelay) Read(destination []byte) (int, error) {
	for len(relay.current) == 0 {
		chunk, ok := <-relay.chunks
		if !ok {
			if relay.err != nil {
				return 0, relay.err
			}
			return 0, io.EOF
		}
		if chunk.err != nil {
			relay.err = chunk.err
			continue
		}
		relay.current = chunk.data
	}
	count := copy(destination, relay.current)
	relay.current = relay.current[count:]
	return count, nil
}

func (relay *boundedRelay) Close() { relay.once.Do(func() { close(relay.stop) }) }

func normalizeBufferSize(value int64) int {
	if value < domaininstance.MinTransferBufferBytes || value > domaininstance.MaxTransferBufferBytes {
		return int(domaininstance.DefaultTransferBufferBytes)
	}
	return int(value)
}

func copyWithBuffer(ctx context.Context, destination io.Writer, source io.Reader, provider BufferSizeProvider) (int64, error) {
	size := int(domaininstance.DefaultTransferBufferBytes)
	if provider != nil {
		size = provider(ctx)
	}
	return io.CopyBuffer(destination, source, make([]byte, size))
}
