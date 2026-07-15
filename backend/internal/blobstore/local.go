package blobstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LocalStore struct {
	root   string
	prefix string
}

func NewLocal(root, prefix string) (*LocalStore, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("local blobstore root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0750); err != nil {
		return nil, err
	}
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix != "" {
		if err := ValidateKey(prefix); err != nil {
			return nil, fmt.Errorf("invalid local storage prefix: %w", err)
		}
	}
	return &LocalStore{root: abs, prefix: prefix}, nil
}

func (s *LocalStore) Provider() string { return ProviderLocal }

func (s *LocalStore) Put(ctx context.Context, key string, r io.Reader, opts PutOptions) (ObjectInfo, error) {
	_ = ctx
	path, key, err := s.pathForKey(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return ObjectInfo{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if err != nil {
		return ObjectInfo{}, err
	}
	tmpPath := tmp.Name()
	h := sha256.New()
	n, copyErr := io.Copy(tmp, io.TeeReader(r, h))
	closeErr := tmp.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return ObjectInfo{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return ObjectInfo{}, closeErr
	}
	if err := os.Chmod(tmpPath, 0640); err != nil {
		_ = os.Remove(tmpPath)
		return ObjectInfo{}, err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return ObjectInfo{}, err
	}
	return ObjectInfo{Key: key, Provider: ProviderLocal, Size: n, ContentType: opts.ContentType, SHA256: hex.EncodeToString(h.Sum(nil)), LastModified: time.Now().UTC()}, nil
}

func (s *LocalStore) PutFile(ctx context.Context, key, path string, opts PutOptions) (ObjectInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return ObjectInfo{}, err
	}
	defer f.Close()
	if opts.ContentType == "" {
		opts.ContentType = DetectContentType(path, "")
	}
	return s.Put(ctx, key, f, opts)
}

func (s *LocalStore) Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	_ = ctx
	path, key, err := s.pathForKey(key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ObjectInfo{}, ErrNotFound
	}
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, ObjectInfo{}, err
	}
	return f, ObjectInfo{Key: key, Provider: ProviderLocal, Size: info.Size(), LastModified: info.ModTime(), ContentType: DetectContentType(path, "")}, nil
}

func (s *LocalStore) Materialize(ctx context.Context, key, dir, filename string) (string, func(), ObjectInfo, error) {
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

func (s *LocalStore) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	_ = ctx
	path, key, err := s.pathForKey(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return ObjectInfo{}, ErrNotFound
	}
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{Key: key, Provider: ProviderLocal, Size: info.Size(), LastModified: info.ModTime(), ContentType: DetectContentType(path, "")}, nil
}

func (s *LocalStore) Delete(ctx context.Context, key string) error {
	_ = ctx
	path, _, err := s.pathForKey(key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *LocalStore) PresignGet(ctx context.Context, key string, opts PresignOptions) (string, error) {
	_ = ctx
	_ = key
	_ = opts
	return "", ErrUnsupported
}

func (s *LocalStore) pathForKey(key string) (string, string, error) {
	key = strings.Trim(key, "/")
	if s.prefix != "" && !strings.HasPrefix(key, s.prefix+"/") {
		key = s.prefix + "/" + key
	}
	if err := ValidateKey(key); err != nil {
		return "", "", err
	}
	path := filepath.Join(s.root, filepath.FromSlash(key))
	rootAbs, err := filepath.Abs(s.root)
	if err != nil {
		return "", "", err
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return "", "", err
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return "", "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", "", ErrBadKey
	}
	return pathAbs, key, nil
}
