package transfer

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Compatible works with AWS S3 and MinIO. URI is s3://bucket/prefix and
// configuration accepts endpoint, region, accessKey, secretKey, sessionToken
// and secure ("true" by default for AWS; false for a local MinIO endpoint).
type S3Credentials struct{ AccessKey, SecretKey, SessionToken string }
type S3CredentialResolver interface {
	Resolve(string) (S3Credentials, error)
}
type EnvironmentS3Credentials struct{}

func (EnvironmentS3Credentials) Resolve(ref string) (S3Credentials, error) {
	if ref != "" && ref != "env" {
		return S3Credentials{}, fmt.Errorf("S3 credential reference %q is not configured", ref)
	}
	key, secret := os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY")
	if key == "" || secret == "" {
		return S3Credentials{}, fmt.Errorf("S3 credentials are required through credential resolver")
	}
	return S3Credentials{key, secret, os.Getenv("AWS_SESSION_TOKEN")}, nil
}

type S3Compatible struct{ Credentials S3CredentialResolver }

func (S3Compatible) CanHandle(e domain.TransferEndpoint) bool {
	return strings.HasPrefix(e.URI, "s3://")
}
func (s S3Compatible) s3Client(e domain.TransferEndpoint) (*minio.Client, string, string, error) {
	u, err := url.Parse(e.URI)
	if err != nil {
		return nil, "", "", err
	}
	if u.Scheme != "s3" || u.Host == "" {
		return nil, "", "", fmt.Errorf("S3 endpoint URI must be s3://bucket/prefix")
	}
	endpoint := e.Configuration["endpoint"]
	if endpoint == "" {
		endpoint = "s3.amazonaws.com"
	}
	secure := e.Configuration["secure"] != "false"
	resolver := s.Credentials
	if resolver == nil {
		resolver = EnvironmentS3Credentials{}
	}
	values, err := resolver.Resolve(e.Configuration["credentialRef"])
	if err != nil {
		return nil, "", "", err
	}
	creds := credentials.NewStaticV4(values.AccessKey, values.SecretKey, values.SessionToken)
	c, err := minio.New(endpoint, &minio.Options{Creds: creds, Secure: secure, Region: e.Configuration["region"]})
	return c, u.Host, strings.TrimPrefix(u.Path, "/"), err
}
func s3Object(prefix, name string) string {
	key := path.Clean(strings.TrimPrefix(name, "/"))
	if key == "." || key == ".." || strings.HasPrefix(key, "../") {
		return ""
	}
	return path.Join(prefix, key)
}
func (s S3Compatible) Exists(ctx context.Context, e domain.TransferEndpoint, name string) (bool, error) {
	c, b, p, err := s.s3Client(e)
	if err != nil {
		return false, err
	}
	object := s3Object(p, name)
	if object == "" {
		return false, fmt.Errorf("S3 transfer path escapes endpoint")
	}
	_, err = c.StatObject(ctx, b, object, minio.StatObjectOptions{})
	if minio.ToErrorResponse(err).Code == "NoSuchKey" {
		return false, nil
	}
	return err == nil, err
}
func (s S3Compatible) Open(ctx context.Context, e domain.TransferEndpoint, name string, offset int64) (io.ReadCloser, error) {
	c, b, p, err := s.s3Client(e)
	if err != nil {
		return nil, err
	}
	obj, err := c.GetObject(ctx, b, s3Object(p, name), minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		_, err = obj.Seek(offset, io.SeekStart)
	}
	return obj, err
}
func (s S3Compatible) Put(ctx context.Context, e domain.TransferEndpoint, name string, input io.Reader, offset int64) error {
	c, b, p, err := s.s3Client(e)
	if err != nil {
		return err
	}
	if offset > 0 {
		previous, getErr := c.GetObject(ctx, b, s3Object(p, name), minio.GetObjectOptions{})
		if getErr != nil {
			return getErr
		}
		defer previous.Close()
		// PutObject replaces an object. Reconstruct exactly the persisted prefix;
		// streaming the whole existing object would duplicate bytes on resume.
		input = io.MultiReader(io.LimitReader(previous, offset), input)
	}
	_, err = c.PutObject(ctx, b, s3Object(p, name), input, -1, minio.PutObjectOptions{})
	return err
}
func (s S3Compatible) Commit(ctx context.Context, e domain.TransferEndpoint, partial, final string) error {
	c, b, p, err := s.s3Client(e)
	if err != nil {
		return err
	}
	src := minio.CopySrcOptions{Bucket: b, Object: s3Object(p, partial)}
	dst := minio.CopyDestOptions{Bucket: b, Object: s3Object(p, final)}
	if _, err = c.CopyObject(ctx, dst, src); err != nil {
		return err
	}
	return c.RemoveObject(ctx, b, s3Object(p, partial), minio.RemoveObjectOptions{})
}
