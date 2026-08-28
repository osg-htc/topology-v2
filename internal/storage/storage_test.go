package storage

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestNew_RequiresBucket(t *testing.T) {
	if _, err := New(context.Background(), Options{}); err == nil {
		t.Error("New with no bucket configured succeeded, want an error")
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// testOptions builds Options against a real S3-compatible endpoint (this dev
// environment's docker-compose MinIO), gated on TOPOLOGY_TEST_S3_ENDPOINT so
// `go test ./...` stays green without one. bucket lets each test avoid
// tripping over another test's objects/buckets.
func testOptions(t *testing.T, bucket string) Options {
	t.Helper()
	endpoint := os.Getenv("TOPOLOGY_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("set TOPOLOGY_TEST_S3_ENDPOINT to run storage tests against a real S3-compatible endpoint")
	}
	return Options{
		Endpoint: endpoint, Region: "us-east-1", Bucket: bucket,
		AccessKey:    envOr("TOPOLOGY_TEST_S3_ACCESS_KEY", "minioadmin"),
		SecretKey:    envOr("TOPOLOGY_TEST_S3_SECRET_KEY", "minioadmin"),
		UsePathStyle: true,
	}
}

func TestStore_UploadDownloadListRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, err := New(ctx, testOptions(t, "topology-test-storage"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	key := fmt.Sprintf("regtest/%d.txt", time.Now().UnixNano())
	content := []byte("hello from the storage test")
	if err := s.Upload(ctx, key, content, "text/plain"); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	got, err := s.Download(ctx, key)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("round trip: got %q, want %q", got, content)
	}

	keys, err := s.ListKeys(ctx, "regtest/")
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	var found bool
	for _, k := range keys {
		if k == key {
			found = true
		}
	}
	if !found {
		t.Errorf("ListKeys(\"regtest/\") = %v, want it to include %q", keys, key)
	}

	if _, err := s.Download(ctx, "regtest/does-not-exist"); err == nil {
		t.Error("Download of a nonexistent key succeeded, want an error")
	}
}

// TestNew_BucketAlreadyExists_Idempotent confirms building a second Store
// against a bucket the first New already created doesn't error -- New
// (via ensureBucket) must tolerate "bucket already owned by me", which a
// real restart of the app hits on every single boot.
func TestNew_BucketAlreadyExists_Idempotent(t *testing.T) {
	ctx := context.Background()
	opts := testOptions(t, "topology-test-storage-idempotent")
	if _, err := New(ctx, opts); err != nil {
		t.Fatalf("first New: %v", err)
	}
	if _, err := New(ctx, opts); err != nil {
		t.Fatalf("second New against the same bucket: %v (ensureBucket should tolerate an already-owned bucket)", err)
	}
}
