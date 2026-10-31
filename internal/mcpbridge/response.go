package mcpbridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (b *Bridge) response(ctx context.Context, response *http.Response, tool string) *mcp.CallToolResult {
	media := response.Header.Get("Content-Type")
	isJSON := strings.Contains(media, "json")
	textual := isJSON || strings.HasPrefix(media, "text/") || media == "" || response.StatusCode >= 300
	reader := response.Body
	var pipe *io.PipeReader
	if textual {
		var writer *io.PipeWriter
		pipe, writer = io.Pipe()
		defer pipe.Close()
		go func() {
			var err error
			if isJSON {
				err = b.sanitizeJSON(response.Body, writer)
			} else {
				err = b.sanitizeText(response.Body, writer)
			}
			writer.CloseWithError(err)
		}()
		reader = pipe
	}
	id, size, err := b.transfers.save(reader)
	if err != nil {
		return failure("daemon response could not be read within transfer limits")
	}
	b.transfers.setOwner(id, identityFromContext(ctx).principal)
	b.transfers.markDownload(id)
	if textual && size <= 1<<20 {
		file, err := b.transfers.open(id)
		if err != nil {
			return failure("could not open response")
		}
		data, err := io.ReadAll(file)
		file.Close()
		b.transfers.remove(id)
		if err != nil {
			return failure("could not read response")
		}
		var value any
		if !isJSON || json.Unmarshal(data, &value) != nil {
			value = string(data)
		}
		return output(map[string]any{"status": response.StatusCode, "data": value, "truncated": false}, response.StatusCode >= 300)
	}
	return output(map[string]any{
		"status": response.StatusCode, "downloadId": id, "size": size, "mediaType": media, "tool": tool,
		"notice": "Use read_transfer for the complete response, then release_transfer.",
	}, response.StatusCode >= 300)
}

func (b *Bridge) sanitizeText(reader io.Reader, writer io.Writer) error {
	scanner := bufio.NewScanner(io.LimitReader(reader, maxTransfer+1))
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	scanner.Split(keepLineEnding)
	for scanner.Scan() {
		if _, err := io.WriteString(writer, b.redactor.text(scanner.Text())); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (b *Bridge) sanitizeJSON(reader io.Reader, writer io.Writer) error {
	decoder := json.NewDecoder(io.LimitReader(reader, maxTransfer+1))
	decoder.UseNumber()
	if err := b.writeJSONValue(decoder, writer, 0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("unexpected trailing JSON")
	}
	return nil
}

func (b *Bridge) writeJSONValue(decoder *json.Decoder, writer io.Writer, depth int) error {
	if depth > 100 {
		return errors.New("response nesting exceeds limit")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, container := token.(json.Delim)
	if !container {
		if text, ok := token.(string); ok {
			token = b.redactor.text(text)
		}
		return json.NewEncoder(writer).Encode(token)
	}
	if _, err = io.WriteString(writer, string(delimiter)); err != nil {
		return err
	}
	first := true
	for decoder.More() {
		if !first {
			if _, err = io.WriteString(writer, ","); err != nil {
				return err
			}
		}
		first = false
		if delimiter == '{' {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("invalid JSON object key")
			}
			if err = json.NewEncoder(writer).Encode(key); err != nil {
				return err
			}
			if _, err = io.WriteString(writer, ":"); err != nil {
				return err
			}
			if sensitive(key) {
				if err = skipJSONValue(decoder); err != nil {
					return err
				}
				if err = json.NewEncoder(writer).Encode("[REDACTED]"); err != nil {
					return err
				}
				continue
			}
		}
		if err = b.writeJSONValue(decoder, writer, depth+1); err != nil {
			return err
		}
	}
	end, err := decoder.Token()
	if err != nil {
		return err
	}
	closing, ok := end.(json.Delim)
	if !ok {
		return errors.New("invalid JSON delimiter")
	}
	_, err = io.WriteString(writer, string(closing))
	return err
}

func skipJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if _, ok := token.(json.Delim); !ok {
		return nil
	}
	depth := 1
	for depth > 0 {
		token, err = decoder.Token()
		if err != nil {
			return err
		}
		if delimiter, ok := token.(json.Delim); ok {
			if delimiter == '{' || delimiter == '[' {
				depth++
			} else {
				depth--
			}
			if depth > 100 {
				return errors.New("response nesting exceeds limit")
			}
		}
	}
	return nil
}

func keepLineEnding(data []byte, atEOF bool) (int, []byte, error) {
	if index := bytes.IndexByte(data, '\n'); index >= 0 {
		return index + 1, data[:index+1], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}
