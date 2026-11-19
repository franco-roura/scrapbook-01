package preview

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Store keeps our copy of preview images. Platform image links (Instagram,
// TikTok) are signed and expire within days, so the original can't be kept.
type Store interface {
	// Put saves a JPEG under key and returns its public URL.
	Put(ctx context.Context, key string, jpeg []byte) (string, error)
}

// S3Store writes to an S3 bucket served at BaseURL (a CloudFront domain).
type S3Store struct {
	client  *s3.Client
	bucket  string
	baseURL string
}

// NewS3Store uses the process's AWS credentials and region.
func NewS3Store(ctx context.Context, bucket, baseURL string) (*S3Store, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("preview: AWS config: %w", err)
	}
	return &S3Store{client: s3.NewFromConfig(cfg), bucket: bucket, baseURL: strings.TrimRight(baseURL, "/")}, nil
}

func (s *S3Store) Put(ctx context.Context, key string, jpeg []byte) (string, error) {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(jpeg),
		ContentType: aws.String("image/jpeg"),
		// Keys are per item and never rewritten with other content.
		CacheControl: aws.String("public, max-age=31536000, immutable"),
	})
	if err != nil {
		return "", fmt.Errorf("preview: put %s: %w", key, err)
	}
	return s.baseURL + "/" + key, nil
}
