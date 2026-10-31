package mcpbridge

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxChunk = 512 << 10
const maxTransfer = int64(8 << 30)
const maxStaged = int64(16 << 30)
const maxTransfers = 32

type transfer struct {
	owner    string
	download bool
	path     string
	size     int64
	sealed   bool
	expires  time.Time
}
type transferStore struct {
	mu     sync.Mutex
	root   string
	items  map[string]*transfer
	closed bool
}

func newTransferStore(directory string) (*transferStore, error) {
	directory, err := stateDirectory(directory)
	if err != nil {
		return nil, err
	}
	root, err := os.MkdirTemp(directory, "transfers-")
	if err != nil {
		return nil, err
	}
	return &transferStore{root: root, items: map[string]*transfer{}}, nil
}

func randomID() string {
	var data [24]byte
	if _, err := rand.Read(data[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(data[:])
}

func (s *transferStore) cleanup() {
	now := time.Now()
	for id, item := range s.items {
		if now.After(item.expires) {
			os.Remove(item.path)
			delete(s.items, id)
		}
	}
}

func (s *transferStore) create() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanup()
	if s.closed || len(s.items) >= maxTransfers {
		return "", errors.New("transfer capacity reached")
	}
	id := randomID()
	path := filepath.Join(s.root, id)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return "", err
	}
	file.Close()
	s.items[id] = &transfer{path: path, expires: time.Now().Add(time.Hour)}
	return id, nil
}

func (s *transferStore) append(id string, offset int64, data []byte, seal bool) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanup()
	item := s.items[id]
	if item == nil || item.sealed {
		return 0, errors.New("upload not found, expired or already sealed")
	}
	if len(data) > maxChunk || offset < 0 {
		return 0, errors.New("invalid chunk")
	}
	if offset != item.size {
		// An identical retransmission is safe; a conflicting one is rejected.
		if offset+int64(len(data)) <= item.size {
			file, err := os.Open(item.path)
			if err != nil {
				return 0, err
			}
			defer file.Close()
			previous := make([]byte, len(data))
			_, err = file.ReadAt(previous, offset)
			if err == nil && string(previous) == string(data) {
				return item.size, nil
			}
		}
		return 0, errors.New("chunk offset conflicts with uploaded bytes")
	}
	if item.size+int64(len(data)) > maxTransfer {
		return 0, errors.New("upload exceeds 8 GiB limit")
	}
	var total int64
	for _, t := range s.items {
		total += t.size
	}
	if total+int64(len(data)) > maxStaged {
		return 0, errors.New("staging quota exceeded")
	}
	file, err := os.OpenFile(item.path, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return 0, err
	}
	n, err := file.Write(data)
	file.Close()
	item.size += int64(n)
	if err == nil {
		item.sealed = seal
		item.expires = time.Now().Add(time.Hour)
	}
	return item.size, err
}

func (s *transferStore) open(id string) (*os.File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanup()
	item := s.items[id]
	if item == nil || !item.sealed {
		return nil, errors.New("sealed transfer not found or expired")
	}
	return os.Open(item.path)
}

func (s *transferStore) read(id string, offset int64, length int) ([]byte, int64, error) {
	if offset < 0 || length < 1 || length > maxChunk {
		return nil, 0, errors.New("invalid offset or chunk length")
	}
	file, err := s.openDownload(id)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, 0, err
	}
	if offset > info.Size() {
		return nil, 0, errors.New("offset exceeds transfer size")
	}
	data := make([]byte, length)
	n, err := file.ReadAt(data, offset)
	if err != nil && err != io.EOF {
		return nil, 0, err
	}
	return data[:n], info.Size(), nil
}

func (s *transferStore) save(reader io.Reader) (string, int64, error) {
	id, err := s.create()
	if err != nil {
		return "", 0, err
	}
	data := make([]byte, maxChunk)
	var size int64
	for {
		n, readErr := reader.Read(data)
		if n > 0 {
			size, err = s.append(id, size, data[:n], false)
			if err != nil {
				s.remove(id)
				return "", 0, err
			}
		}
		if readErr == io.EOF {
			_, err = s.append(id, size, nil, true)
			return id, size, err
		}
		if readErr != nil {
			s.remove(id)
			return "", 0, readErr
		}
	}
}

func (s *transferStore) remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if item := s.items[id]; item != nil {
		os.Remove(item.path)
		delete(s.items, id)
	}
}

func (s *transferStore) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	os.RemoveAll(s.root)
	s.items = map[string]*transfer{}
}

func multipartUpload(file *os.File) (io.Reader, string, func(), error) {
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	go func() {
		part, err := multipartWriter.CreateFormFile("context", "context.tar.gz")
		if err == nil {
			_, err = io.Copy(part, file)
		}
		if err == nil {
			err = multipartWriter.Close()
		}
		writer.CloseWithError(err)
	}()
	return reader, multipartWriter.FormDataContentType(), func() { reader.Close(); file.Close() }, nil
}

func (b *Bridge) addTransferTools() {
	if b.options.Permissions.Allows(Write) {
		b.addUploadTools()
	}
	b.addDownloadTools()
}

func (b *Bridge) addUploadTools() {
	type beginInput struct{}
	mcp.AddTool(b.server, &mcp.Tool{
		Name:        "begin_upload",
		Description: "Create a temporary upload. Send base64 chunks with upload_chunk, then seal_upload. No filesystem paths or remote URLs are accepted.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in beginInput) (*mcp.CallToolResult, any, error) {
		id, err := b.transfers.create()
		if err != nil {
			return failure(err.Error()), nil, nil
		}
		b.transfers.setOwner(id, identityFromContext(requestContext(ctx, req)).principal)
		return nil, map[string]any{"uploadId": id, "maxChunkBytes": maxChunk, "maxBytes": maxTransfer, "expiresInSeconds": 3600}, nil
	})
	type chunkInput struct {
		UploadID string `json:"uploadId"`
		Offset   int64  `json:"offset"`
		Data     string `json:"data" jsonschema:"base64 encoded bytes, at most 512 KiB decoded"`
	}
	mcp.AddTool(b.server, &mcp.Tool{
		Name:        "upload_chunk",
		Description: "Append bytes at the exact upload offset. Identical chunk retransmissions are accepted.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in chunkInput) (*mcp.CallToolResult, any, error) {
		if !b.ownsTransfer(in.UploadID, req) {
			return failure("upload not found or not accessible"), nil, nil
		}
		if len(in.Data) > base64.StdEncoding.EncodedLen(maxChunk) {
			return failure("chunk exceeds limit"), nil, nil
		}
		data, err := base64.StdEncoding.DecodeString(in.Data)
		if err != nil {
			return failure("invalid base64"), nil, nil
		}
		size, err := b.transfers.append(in.UploadID, in.Offset, data, false)
		if err != nil {
			return failure(err.Error()), nil, nil
		}
		return nil, map[string]any{"size": size}, nil
	})
	type sealInput struct {
		UploadID string `json:"uploadId"`
		Size     int64  `json:"size"`
	}
	mcp.AddTool(b.server, &mcp.Tool{
		Name:        "seal_upload",
		Description: "Seal an upload at its expected size, making it immutable and ready for a domain tool's uploadId argument.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in sealInput) (*mcp.CallToolResult, any, error) {
		if !b.ownsTransfer(in.UploadID, req) {
			return failure("upload not found or not accessible"), nil, nil
		}
		file, err := b.transfers.open(in.UploadID)
		if err == nil {
			info, e := file.Stat()
			file.Close()
			if e == nil && info.Size() == in.Size {
				return nil, map[string]any{"uploadId": in.UploadID, "size": in.Size}, nil
			}
		}
		size, err := b.transfers.append(in.UploadID, in.Size, nil, true)
		if err != nil {
			return failure(err.Error()), nil, nil
		}
		return nil, map[string]any{"uploadId": in.UploadID, "size": size}, nil
	})
}

func (b *Bridge) addDownloadTools() {
	type readInput struct {
		DownloadID string `json:"downloadId"`
		Offset     int64  `json:"offset"`
		Length     int    `json:"length" jsonschema:"number of bytes, between 1 and 524288"`
	}
	mcp.AddTool(b.server, &mcp.Tool{
		Name:        "read_transfer",
		Description: "Read a bounded base64 chunk from a staged download. References expire after one hour.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, in readInput) (*mcp.CallToolResult, any, error) {
		if !b.ownsTransfer(in.DownloadID, req) {
			return failure("download not found or not accessible"), nil, nil
		}
		data, size, err := b.transfers.read(in.DownloadID, in.Offset, in.Length)
		if err != nil {
			return failure(err.Error()), nil, nil
		}
		return nil, map[string]any{
			"data":       base64.StdEncoding.EncodeToString(data),
			"offset":     in.Offset,
			"nextOffset": in.Offset + int64(len(data)),
			"size":       size,
			"eof":        in.Offset+int64(len(data)) >= size,
		}, nil
	})
	type releaseInput struct {
		TransferID string `json:"transferId"`
	}
	mcp.AddTool(b.server, &mcp.Tool{
		Name:        "release_transfer",
		Description: "Delete a temporary upload or download reference and release staging space.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in releaseInput) (*mcp.CallToolResult, any, error) {
		if !b.ownsTransfer(in.TransferID, req) {
			return failure("transfer not found or not accessible"), nil, nil
		}
		b.transfers.remove(in.TransferID)
		return nil, map[string]any{"released": true}, nil
	})
}

func (s *transferStore) markDownload(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if item := s.items[id]; item != nil {
		item.download = true
	}
}

func (s *transferStore) openDownload(id string) (*os.File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanup()
	item := s.items[id]
	if item == nil || !item.sealed || !item.download {
		return nil, errors.New("download not found or expired")
	}
	return os.Open(item.path)
}
