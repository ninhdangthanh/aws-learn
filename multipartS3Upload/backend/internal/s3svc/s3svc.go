package s3svc

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Service wraps the S3 client with the multipart operations the backend needs.
// The backend never touches file bytes — it only orchestrates the upload and
// hands out presigned URLs so the browser uploads directly to S3.
type Service struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
	expiry  time.Duration
}

// CompletedPart pairs a part number with the ETag S3 returned for that part.
type CompletedPart struct {
	PartNumber int32
	ETag       string
}

// New builds a Service from the default AWS credential chain (env vars, shared
// config file, or the instance/role profile).
func New(ctx context.Context, region, bucket string, expiry time.Duration) (*Service, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg)
	return &Service{
		client:  client,
		presign: s3.NewPresignClient(client),
		bucket:  bucket,
		expiry:  expiry,
	}, nil
}

// CreateMultipartUpload starts a multipart upload and returns the uploadId that
// ties all subsequent part/complete/abort calls together.
func (s *Service) CreateMultipartUpload(ctx context.Context, key, contentType string) (string, error) {
	out, err := s.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", err
	}
	return aws.ToString(out.UploadId), nil
}

// PresignUploadPart returns a short-lived URL the browser can PUT a single part to.
func (s *Service) PresignUploadPart(ctx context.Context, key, uploadID string, partNumber int32) (string, error) {
	req, err := s.presign.PresignUploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(s.bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(partNumber),
	}, s3.WithPresignExpires(s.expiry))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// CompleteMultipartUpload asks S3 to assemble the uploaded parts into the final
// object. Parts must be provided in ascending part-number order.
func (s *Service) CompleteMultipartUpload(ctx context.Context, key, uploadID string, parts []CompletedPart) (string, error) {
	completed := make([]types.CompletedPart, len(parts))
	for i, p := range parts {
		completed[i] = types.CompletedPart{
			PartNumber: aws.Int32(p.PartNumber),
			ETag:       aws.String(p.ETag),
		}
	}
	out, err := s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(s.bucket),
		Key:             aws.String(key),
		UploadId:        aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: completed},
	})
	if err != nil {
		return "", err
	}
	return aws.ToString(out.Location), nil
}

// AbortMultipartUpload cancels an upload and lets S3 reclaim any uploaded parts.
func (s *Service) AbortMultipartUpload(ctx context.Context, key, uploadID string) error {
	_, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(s.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	return err
}
