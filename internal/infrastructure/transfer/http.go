package transfer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// HTTPDownload supports public/presigned HTTP(S) inputs. Upload requires an
// explicit writable connector (S3, filesystem or SSH) rather than guessing a
// server-specific PUT protocol.
type HTTPDownload struct{ Client *http.Client }

func (h HTTPDownload) client() *http.Client {
	if h.Client != nil {
		return h.Client
	}
	return http.DefaultClient
}
func (HTTPDownload) CanHandle(e domain.TransferEndpoint) bool {
	u, err := url.Parse(e.URI)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}
func (h HTTPDownload) Exists(ctx context.Context, e domain.TransferEndpoint, _ string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, e.URI, nil)
	if err != nil {
		return false, err
	}
	res, err := h.client().Do(req)
	if err != nil {
		return false, err
	}
	res.Body.Close()
	return res.StatusCode >= 200 && res.StatusCode < 300, nil
}
func (h HTTPDownload) Open(ctx context.Context, e domain.TransferEndpoint, _ string, offset int64) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.URI, nil)
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	res, err := h.client().Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		res.Body.Close()
		return nil, fmt.Errorf("HTTP download %s: %s", e.URI, res.Status)
	}
	return res.Body, nil
}
func (HTTPDownload) Put(context.Context, domain.TransferEndpoint, string, io.Reader, int64) error {
	return fmt.Errorf("HTTP endpoint is read-only; use S3, SSH or filesystem destination")
}
func (HTTPDownload) Commit(context.Context, domain.TransferEndpoint, string, string) error {
	return fmt.Errorf("HTTP endpoint is read-only")
}
