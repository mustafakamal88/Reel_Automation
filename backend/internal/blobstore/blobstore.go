package blobstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	ProviderLocal = "local"
	ProviderS3    = "s3"
)

var (
	ErrNotFound    = errors.New("blobstore: object not found")
	ErrBadKey      = errors.New("blobstore: invalid object key")
	ErrUnsupported = errors.New("blobstore: operation unsupported by provider")
)

type Store interface {
	Put(ctx context.Context, key string, r io.Reader, opts PutOptions) (ObjectInfo, error)
	PutFile(ctx context.Context, key, path string, opts PutOptions) (ObjectInfo, error)
	Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
	Materialize(ctx context.Context, key, dir, filename string) (string, func(), ObjectInfo, error)
	Stat(ctx context.Context, key string) (ObjectInfo, error)
	Delete(ctx context.Context, key string) error
	PresignGet(ctx context.Context, key string, opts PresignOptions) (string, error)
	Provider() string
}

type PutOptions struct {
	ContentType        string
	ContentDisposition string
	OriginalFilename   string
}

type PresignOptions struct {
	TTL                time.Duration
	ContentDisposition string
}

type ObjectInfo struct {
	Key          string
	Provider     string
	Size         int64
	ContentType  string
	ETag         string
	SHA256       string
	LastModified time.Time
}

type Config struct {
	Provider        string
	LocalDir        string
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	ForcePathStyle  bool
	Prefix          string
	SignedURLTTL    time.Duration
}

func New(ctx context.Context, cfg Config) (Store, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = ProviderLocal
	}
	switch provider {
	case ProviderLocal:
		if strings.TrimSpace(cfg.LocalDir) == "" {
			return nil, errors.New("MEDIA_STORAGE_LOCAL_DIR is required for local media storage")
		}
		return NewLocal(cfg.LocalDir, cfg.Prefix)
	case ProviderS3:
		return NewS3(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported MEDIA_STORAGE_PROVIDER %q", provider)
	}
}

func ValidateKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" || strings.HasPrefix(key, "/") || strings.Contains(key, "\\") {
		return ErrBadKey
	}
	parts := strings.Split(key, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return ErrBadKey
		}
	}
	return nil
}

func JoinKey(parts ...string) (string, error) {
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, "/")
		if part == "" {
			continue
		}
		for _, sub := range strings.Split(part, "/") {
			if err := ValidateKey(sub); err != nil {
				return "", err
			}
			clean = append(clean, sub)
		}
	}
	key := strings.Join(clean, "/")
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	return key, nil
}

func SafeSegment(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-_.")
	if out == "" {
		return fallback
	}
	return out
}

func HashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func DetectContentType(path, fallback string) string {
	if fallback = strings.TrimSpace(fallback); fallback != "" {
		if idx := strings.Index(fallback, ";"); idx >= 0 {
			fallback = fallback[:idx]
		}
		return strings.ToLower(strings.TrimSpace(fallback))
	}
	if ext := strings.ToLower(filepath.Ext(path)); ext != "" {
		if ct := mime.TypeByExtension(ext); ct != "" {
			if idx := strings.Index(ct, ";"); idx >= 0 {
				ct = ct[:idx]
			}
			return strings.ToLower(strings.TrimSpace(ct))
		}
	}
	return "application/octet-stream"
}

func safeMaterializePath(dir, filename string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", errors.New("materialize dir is required")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	name := filepath.Base(strings.TrimSpace(filename))
	if name == "." || name == "/" || name == "" {
		name = "object"
	}
	path := filepath.Join(dir, name)
	baseAbs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(baseAbs, pathAbs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", ErrBadKey
	}
	return pathAbs, nil
}
