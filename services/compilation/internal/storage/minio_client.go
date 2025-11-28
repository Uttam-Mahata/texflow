package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

// MinIOClient wraps MinIO client with additional functionality
type MinIOClient struct {
	client         *minio.Client
	publicClient   *minio.Client // Client configured with public endpoint for presigned URLs
	bucket         string
	logger         *zap.Logger
}

// NewMinIOClient creates a new MinIO client
func NewMinIOClient(endpoint, publicEndpoint, accessKey, secretKey, bucket string, useSSL bool, logger *zap.Logger) (*MinIOClient, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// Create a separate client with the public endpoint for presigned URLs
	// This is necessary when running in Docker where internal and external hostnames differ
	publicClient := client
	if publicEndpoint != "" && publicEndpoint != endpoint {
		publicClient, err = minio.New(publicEndpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
			Secure: useSSL,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create public MinIO client: %w", err)
		}
		logger.Info("Created separate public MinIO client for presigned URLs",
			zap.String("internal", endpoint),
			zap.String("public", publicEndpoint),
		)
	}

	m := &MinIOClient{
		client:       client,
		publicClient: publicClient,
		bucket:       bucket,
		logger:       logger,
	}

	// Ensure bucket exists
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		err = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
		logger.Info("Created MinIO bucket", zap.String("bucket", bucket))
	}

	return m, nil
}

// UploadFile uploads a file to MinIO
func (m *MinIOClient) UploadFile(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error {
	_, err := m.client.PutObject(
		ctx,
		m.bucket,
		objectName,
		reader,
		size,
		minio.PutObjectOptions{
			ContentType: contentType,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	m.logger.Debug("File uploaded to MinIO",
		zap.String("bucket", m.bucket),
		zap.String("object", objectName),
		zap.Int64("size", size),
	)

	return nil
}

// UploadBytes uploads byte data to MinIO
func (m *MinIOClient) UploadBytes(ctx context.Context, objectName string, data []byte, contentType string) error {
	reader := bytes.NewReader(data)
	return m.UploadFile(ctx, objectName, reader, int64(len(data)), contentType)
}

// DownloadFile downloads a file from MinIO
func (m *MinIOClient) DownloadFile(ctx context.Context, objectName string) (*minio.Object, error) {
	object, err := m.client.GetObject(ctx, m.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}

	return object, nil
}

// DownloadBytes downloads a file as bytes
func (m *MinIOClient) DownloadBytes(ctx context.Context, objectName string) ([]byte, error) {
	object, err := m.DownloadFile(ctx, objectName)
	if err != nil {
		return nil, err
	}
	defer object.Close()

	data, err := io.ReadAll(object)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return data, nil
}

// DeleteFile deletes a file from MinIO
func (m *MinIOClient) DeleteFile(ctx context.Context, objectName string) error {
	err := m.client.RemoveObject(ctx, m.bucket, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	m.logger.Debug("File deleted from MinIO",
		zap.String("bucket", m.bucket),
		zap.String("object", objectName),
	)

	return nil
}

// GeneratePresignedURL generates a presigned URL for file download
// Uses the public client so the URL is accessible from browsers
func (m *MinIOClient) GeneratePresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	url, err := m.publicClient.PresignedGetObject(ctx, m.bucket, objectName, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return url.String(), nil
}

// ListObjects lists objects with a given prefix
func (m *MinIOClient) ListObjects(ctx context.Context, prefix string) ([]minio.ObjectInfo, error) {
	var objects []minio.ObjectInfo

	objectCh := m.client.ListObjects(ctx, m.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", object.Err)
		}
		objects = append(objects, object)
	}

	return objects, nil
}

// CopyFile copies a file within MinIO
func (m *MinIOClient) CopyFile(ctx context.Context, sourceObject, destObject string) error {
	src := minio.CopySrcOptions{
		Bucket: m.bucket,
		Object: sourceObject,
	}

	dst := minio.CopyDestOptions{
		Bucket: m.bucket,
		Object: destObject,
	}

	_, err := m.client.CopyObject(ctx, dst, src)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// FileExists checks if a file exists in MinIO
func (m *MinIOClient) FileExists(ctx context.Context, objectName string) (bool, error) {
	_, err := m.client.StatObject(ctx, m.bucket, objectName, minio.StatObjectOptions{})
	if err != nil {
		errResponse := minio.ToErrorResponse(err)
		if errResponse.Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
