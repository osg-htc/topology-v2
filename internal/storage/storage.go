// Package storage wraps an S3-compatible object store (AWS S3 or MinIO) for
// encrypted backup archives. Mirrors the SWAMP Store pattern.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Store is a thin wrapper over the S3 client bound to a single bucket.
type Store struct {
	client *s3.Client
	bucket string
}

// Options configures the S3 client.
type Options struct {
	Endpoint     string
	Region       string
	Bucket       string
	AccessKey    string
	SecretKey    string
	UsePathStyle bool
}

// New builds a Store, verifying (and creating, if missing) the bucket.
func New(ctx context.Context, opts Options) (*Store, error) {
	if opts.Bucket == "" {
		return nil, errors.New("s3 bucket not configured")
	}
	client := s3.New(s3.Options{
		Region:       opts.Region,
		BaseEndpoint: nonEmpty(opts.Endpoint),
		UsePathStyle: opts.UsePathStyle,
		Credentials: credentials.NewStaticCredentialsProvider(
			opts.AccessKey, opts.SecretKey, ""),
	})

	s := &Store{client: client, bucket: opts.Bucket}
	if err := s.ensureBucket(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) ensureBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &s.bucket})
	if err == nil {
		return nil
	}
	_, cerr := s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: &s.bucket})
	if cerr != nil {
		var owned *types.BucketAlreadyOwnedByYou
		if errors.As(cerr, &owned) {
			return nil
		}
		return fmt.Errorf("ensuring bucket %q: %w", s.bucket, cerr)
	}
	return nil
}

// Upload stores bytes at key.
func (s *Store) Upload(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &s.bucket,
		Key:         &key,
		Body:        bytes.NewReader(data),
		ContentType: nonEmpty(contentType),
	})
	if err != nil {
		return fmt.Errorf("uploading %q: %w", key, err)
	}
	return nil
}

// Download fetches bytes at key.
func (s *Store) Download(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		return nil, fmt.Errorf("downloading %q: %w", key, err)
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return aws.String(s)
}
