package storage

import (
	"context"
	"io"
	"net/http"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Config struct {
	Endpoint  string
	Bucket    string
	AccessKey string
	Secret    string
	Region    string
	UseSSL    bool
}

type S3 struct {
	client *minio.Client
	bucket string
}

func NewS3(ctx context.Context, cfg S3Config) (*S3, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.Secret, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, err
	}

	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
			return nil, err
		}
	}
	return &S3{client: client, bucket: cfg.Bucket}, nil
}

func (s *S3) Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error {
	if !ValidKey(key) {
		return ErrBadKey
	}
	if size <= 0 {
		size = -1
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, body, size,
		minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *S3) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if !ValidKey(key) {
		return nil, ErrBadKey
	}
	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}

	if _, err := object.Stat(); err != nil {
		object.Close()
		if minio.ToErrorResponse(err).StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return object, nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	if !ValidKey(key) {
		return ErrBadKey
	}
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}
