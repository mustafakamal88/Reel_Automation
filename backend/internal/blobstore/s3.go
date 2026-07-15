package blobstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Store struct {
	client     *s3.Client
	presign    *s3.PresignClient
	bucket     string
	prefix     string
	defaultTTL time.Duration
}

func NewS3(ctx context.Context, cfg Config) (*S3Store, error) {
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, errors.New("MEDIA_STORAGE_BUCKET is required for s3 media storage")
	}
	if strings.TrimSpace(cfg.Region) == "" {
		return nil, errors.New("MEDIA_STORAGE_REGION is required for s3 media storage")
	}
	if strings.TrimSpace(cfg.AccessKeyID) == "" {
		return nil, errors.New("MEDIA_STORAGE_ACCESS_KEY_ID is required for s3 media storage")
	}
	if strings.TrimSpace(cfg.SecretAccessKey) == "" {
		return nil, errors.New("MEDIA_STORAGE_SECRET_ACCESS_KEY is required for s3 media storage")
	}
	prefix := strings.Trim(strings.TrimSpace(cfg.Prefix), "/")
	if prefix != "" {
		if err := ValidateKey(prefix); err != nil {
			return nil, fmt.Errorf("invalid MEDIA_STORAGE_PREFIX: %w", err)
		}
	}
	ttl := cfg.SignedURLTTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	if ttl < time.Minute || ttl > 24*time.Hour {
		return nil, errors.New("MEDIA_STORAGE_SIGNED_URL_TTL must be between 1m and 24h")
	}
	loadOpts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken)),
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("s3 config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if endpoint := strings.TrimSpace(cfg.Endpoint); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		o.UsePathStyle = cfg.ForcePathStyle
	})
	return &S3Store{
		client:     client,
		presign:    s3.NewPresignClient(client),
		bucket:     cfg.Bucket,
		prefix:     prefix,
		defaultTTL: ttl,
	}, nil
}

func (s *S3Store) Provider() string { return ProviderS3 }

func (s *S3Store) Put(ctx context.Context, key string, r io.Reader, opts PutOptions) (ObjectInfo, error) {
	key, err := s.fullKey(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return ObjectInfo{}, err
	}
	sum := sha256Hex(data)
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
		ACL:    types.ObjectCannedACLPrivate,
	}
	if opts.ContentType != "" {
		input.ContentType = aws.String(opts.ContentType)
	}
	if opts.ContentDisposition != "" {
		input.ContentDisposition = aws.String(opts.ContentDisposition)
	}
	if opts.OriginalFilename != "" {
		input.Metadata = map[string]string{"original-filename": opts.OriginalFilename, "sha256": sum}
	} else {
		input.Metadata = map[string]string{"sha256": sum}
	}
	out, err := s.client.PutObject(ctx, input)
	if err != nil {
		return ObjectInfo{}, sanitizeS3Error(err)
	}
	etag := strings.Trim(aws.ToString(out.ETag), `"`)
	return ObjectInfo{Key: key, Provider: ProviderS3, Size: int64(len(data)), ContentType: opts.ContentType, ETag: etag, SHA256: sum, LastModified: time.Now().UTC()}, nil
}

func (s *S3Store) PutFile(ctx context.Context, key, path string, opts PutOptions) (ObjectInfo, error) {
	key, err := s.fullKey(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	sum, size, err := HashFile(path)
	if err != nil {
		return ObjectInfo{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return ObjectInfo{}, err
	}
	defer f.Close()
	if opts.ContentType == "" {
		opts.ContentType = DetectContentType(path, "")
	}
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   f,
		ACL:    types.ObjectCannedACLPrivate,
		Metadata: map[string]string{
			"sha256": sum,
		},
	}
	if opts.ContentType != "" {
		input.ContentType = aws.String(opts.ContentType)
	}
	if opts.ContentDisposition != "" {
		input.ContentDisposition = aws.String(opts.ContentDisposition)
	}
	if opts.OriginalFilename != "" {
		input.Metadata["original-filename"] = opts.OriginalFilename
	}
	out, err := s.client.PutObject(ctx, input)
	if err != nil {
		return ObjectInfo{}, sanitizeS3Error(err)
	}
	etag := strings.Trim(aws.ToString(out.ETag), `"`)
	return ObjectInfo{Key: key, Provider: ProviderS3, Size: size, ContentType: opts.ContentType, ETag: etag, SHA256: sum, LastModified: time.Now().UTC()}, nil
}

func (s *S3Store) Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	key, err := s.fullKey(key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		if isS3NotFound(err) {
			return nil, ObjectInfo{}, ErrNotFound
		}
		return nil, ObjectInfo{}, sanitizeS3Error(err)
	}
	return out.Body, ObjectInfo{
		Key:          key,
		Provider:     ProviderS3,
		Size:         aws.ToInt64(out.ContentLength),
		ContentType:  aws.ToString(out.ContentType),
		ETag:         strings.Trim(aws.ToString(out.ETag), `"`),
		LastModified: aws.ToTime(out.LastModified),
	}, nil
}

func (s *S3Store) Materialize(ctx context.Context, key, dir, filename string) (string, func(), ObjectInfo, error) {
	src, info, err := s.Open(ctx, key)
	if err != nil {
		return "", nil, ObjectInfo{}, err
	}
	defer src.Close()
	path, err := safeMaterializePath(dir, filename)
	if err != nil {
		return "", nil, ObjectInfo{}, err
	}
	dst, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", nil, ObjectInfo{}, err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		_ = os.Remove(path)
		return "", nil, ObjectInfo{}, err
	}
	if err := dst.Close(); err != nil {
		_ = os.Remove(path)
		return "", nil, ObjectInfo{}, err
	}
	return path, func() { _ = os.Remove(path) }, info, nil
}

func (s *S3Store) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	key, err := s.fullKey(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		if isS3NotFound(err) {
			return ObjectInfo{}, ErrNotFound
		}
		return ObjectInfo{}, sanitizeS3Error(err)
	}
	return ObjectInfo{
		Key:          key,
		Provider:     ProviderS3,
		Size:         aws.ToInt64(out.ContentLength),
		ContentType:  aws.ToString(out.ContentType),
		ETag:         strings.Trim(aws.ToString(out.ETag), `"`),
		LastModified: aws.ToTime(out.LastModified),
	}, nil
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	key, err := s.fullKey(key)
	if err != nil {
		return err
	}
	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return sanitizeS3Error(err)
	}
	return nil
}

func (s *S3Store) PresignGet(ctx context.Context, key string, opts PresignOptions) (string, error) {
	key, err := s.fullKey(key)
	if err != nil {
		return "", err
	}
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	input := &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)}
	if opts.ContentDisposition != "" {
		input.ResponseContentDisposition = aws.String(opts.ContentDisposition)
	}
	out, err := s.presign.PresignGetObject(ctx, input, func(o *s3.PresignOptions) {
		o.Expires = ttl
	})
	if err != nil {
		return "", sanitizeS3Error(err)
	}
	if _, err := url.Parse(out.URL); err != nil {
		return "", errors.New("s3 presign returned invalid URL")
	}
	return out.URL, nil
}

func (s *S3Store) fullKey(key string) (string, error) {
	key = strings.Trim(key, "/")
	if s.prefix != "" && !strings.HasPrefix(key, s.prefix+"/") {
		key = s.prefix + "/" + key
	}
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	return key, nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}

func isS3NotFound(err error) bool {
	var noKey *types.NoSuchKey
	if errors.As(err, &noKey) {
		return true
	}
	var notFound *types.NotFound
	return errors.As(err, &notFound)
}

func sanitizeS3Error(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	for _, marker := range []string{"AWSAccessKeyId", "Credential=", "Signature=", "X-Amz-Signature"} {
		if strings.Contains(msg, marker) {
			return errors.New("s3 storage request failed")
		}
	}
	return fmt.Errorf("s3 storage request failed: %w", err)
}
