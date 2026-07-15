package blobstore

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateKeyRejectsTraversalAndUnsafePaths(t *testing.T) {
	bad := []string{"", "/abs/key", "a//b", "a/../b", "../b", `a\b`, "a/./b"}
	for _, key := range bad {
		if err := ValidateKey(key); err == nil {
			t.Fatalf("ValidateKey(%q) succeeded, want error", key)
		}
	}
	if err := ValidateKey("workspaces/ws/clip-studio/sources/src/source.mp4"); err != nil {
		t.Fatalf("valid key rejected: %v", err)
	}
}

func TestLocalStorePutOpenStatMaterializeDelete(t *testing.T) {
	ctx := context.Background()
	store, err := NewLocal(t.TempDir(), "prefix")
	if err != nil {
		t.Fatalf("new local: %v", err)
	}
	info, err := store.Put(ctx, "workspaces/ws/file.txt", strings.NewReader("hello"), PutOptions{ContentType: "text/plain"})
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if info.Provider != ProviderLocal || info.Size != 5 || len(info.SHA256) != 64 {
		t.Fatalf("unexpected put info: %+v", info)
	}
	body, opened, err := store.Open(ctx, "prefix/workspaces/ws/file.txt")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer body.Close()
	data, _ := io.ReadAll(body)
	if string(data) != "hello" || opened.Size != 5 {
		t.Fatalf("opened data=%q info=%+v", data, opened)
	}
	stat, err := store.Stat(ctx, "workspaces/ws/file.txt")
	if err != nil || stat.Size != 5 {
		t.Fatalf("stat=%+v err=%v", stat, err)
	}
	matDir := t.TempDir()
	path, cleanup, _, err := store.Materialize(ctx, "workspaces/ws/file.txt", matDir, "../safe.txt")
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if filepath.Dir(path) != matDir {
		t.Fatalf("materialize escaped dir: %s", path)
	}
	cleanup()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("materialized file still exists or unexpected err: %v", err)
	}
	if err := store.Delete(ctx, "workspaces/ws/file.txt"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.Stat(ctx, "workspaces/ws/file.txt"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stat after delete err=%v, want ErrNotFound", err)
	}
}

func TestHashFileAndContentType(t *testing.T) {
	path := filepath.Join(t.TempDir(), "video.mp4")
	if err := os.WriteFile(path, []byte("abc"), 0600); err != nil {
		t.Fatal(err)
	}
	sum, size, err := HashFile(path)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if size != 3 || sum != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("hash size=%d sum=%s", size, sum)
	}
	if got := DetectContentType(path, ""); got != "video/mp4" {
		t.Fatalf("content type=%q", got)
	}
}

func TestS3ConfigValidationNoSecretsInErrors(t *testing.T) {
	_, err := NewS3(context.Background(), Config{
		Region:          "auto",
		Bucket:          "bucket",
		AccessKeyID:     "AKIA_TEST_SECRET",
		SecretAccessKey: "super-secret-value",
		SignedURLTTL:    30 * time.Second,
	})
	if err == nil {
		t.Fatal("expected invalid ttl error")
	}
	if strings.Contains(err.Error(), "super-secret-value") || strings.Contains(err.Error(), "AKIA_TEST_SECRET") {
		t.Fatalf("error leaked secret: %v", err)
	}
	_, err = NewS3(context.Background(), Config{Provider: ProviderS3, Region: "auto", Bucket: "bucket"})
	if err == nil || strings.Contains(err.Error(), "secret") && strings.Contains(err.Error(), "value") {
		t.Fatalf("expected safe missing credentials error, got %v", err)
	}
}
