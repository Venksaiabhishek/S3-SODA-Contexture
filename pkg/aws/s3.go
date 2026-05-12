package aws

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// CreateS3Bucket creates a new S3 bucket (or MinIO bucket)
func (c *Client) CreateS3Bucket(ctx context.Context, bucketName string) (*types.AWSResource, error) {
	c.logger.WithField("bucket", bucketName).Info("Creating S3 Bucket")

	_, err := c.s3.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
	}

	return &types.AWSResource{
		ID:    bucketName,
		Type:  "s3_bucket",
		State: "available",
		Details: map[string]interface{}{
			"bucketName": bucketName,
		},
	}, nil
}

// BucketContext contains rich OCS-aligned context for a single bucket
type BucketContext struct {
	Name         string
	CreatedAt    string
	ObjectCount  int
	TotalSize    int64
	SizeHuman    string
	DataFormats  []string
	SampleFiles  []string
}

// ListS3BucketsWithContext lists all buckets with full SODA Contexture metadata
func (c *Client) ListS3BucketsWithContext(ctx context.Context) ([]BucketContext, error) {
	c.logger.Info("Listing S3 Buckets with context")

	output, err := c.s3.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	var results []BucketContext
	for _, bucket := range output.Buckets {
		if bucket.Name == nil {
			continue
		}
		name := *bucket.Name

		bc := BucketContext{
			Name: name,
		}

		// Creation date
		if bucket.CreationDate != nil {
			bc.CreatedAt = bucket.CreationDate.Format(time.RFC3339)
		}

		// Enumerate objects for context
		bc.ObjectCount, bc.TotalSize, bc.DataFormats, bc.SampleFiles = c.gatherBucketStats(ctx, name)
		bc.SizeHuman = humanizeBytes(bc.TotalSize)

		results = append(results, bc)
	}

	return results, nil
}

// ListS3Buckets lists all S3 buckets (basic, for backward compatibility)
func (c *Client) ListS3Buckets(ctx context.Context) ([]*types.AWSResource, error) {
	c.logger.Info("Listing S3 Buckets")

	output, err := c.s3.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	var resources []*types.AWSResource
	for _, bucket := range output.Buckets {
		if bucket.Name != nil {
			name := *bucket.Name
			resources = append(resources, &types.AWSResource{
				ID:    name,
				Type:  "s3_bucket",
				State: "available",
				Details: map[string]interface{}{
					"bucketName": name,
				},
			})
		}
	}
	return resources, nil
}

// DeleteS3Bucket deletes an existing S3 bucket
func (c *Client) DeleteS3Bucket(ctx context.Context, bucketName string) error {
	c.logger.WithField("bucket", bucketName).Info("Deleting S3 Bucket")

	_, err := c.s3.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete bucket %s: %w", bucketName, err)
	}

	return nil
}

// ListS3Objects lists all objects in an existing S3 bucket (keys only, for backward compatibility)
func (c *Client) ListS3Objects(ctx context.Context, bucketName string) ([]string, error) {
	c.logger.WithField("bucket", bucketName).Info("Listing S3 Objects")

	maxKeys := int32(1000)
	listOutput, err := c.s3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucketName),
		MaxKeys: &maxKeys,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list objects in bucket %s: %w", bucketName, err)
	}

	var keys []string
	for _, obj := range listOutput.Contents {
		if obj.Key != nil {
			keys = append(keys, *obj.Key)
		}
	}
	return keys, nil
}

// S3ObjectDetail contains full metadata for a single S3 object
type S3ObjectDetail struct {
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	SizeHuman    string `json:"sizeHuman"`
	LastModified string `json:"lastModified"`
	ETag         string `json:"etag"`
	Prefix       string `json:"prefix"` // extracted namespace/directory from key path
}

// ListS3ObjectsDetailed lists all objects in a bucket with full metadata (size, lastModified, etag, prefix)
func (c *Client) ListS3ObjectsDetailed(ctx context.Context, bucketName string) ([]S3ObjectDetail, error) {
	c.logger.WithField("bucket", bucketName).Info("Listing S3 Objects with full metadata")

	maxKeys := int32(1000)
	listOutput, err := c.s3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucketName),
		MaxKeys: &maxKeys,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list objects in bucket %s: %w", bucketName, err)
	}

	var objects []S3ObjectDetail
	for _, obj := range listOutput.Contents {
		if obj.Key == nil {
			continue
		}

		detail := S3ObjectDetail{
			Key: *obj.Key,
		}

		if obj.Size != nil {
			detail.Size = *obj.Size
			detail.SizeHuman = humanizeBytes(*obj.Size)
		}

		if obj.LastModified != nil {
			detail.LastModified = obj.LastModified.Format(time.RFC3339)
		}

		if obj.ETag != nil {
			detail.ETag = strings.Trim(*obj.ETag, "\"")
		}

		// Extract prefix/namespace from key path (e.g., "logs/2026/jan.log" → "logs/2026/")
		if idx := strings.LastIndex(*obj.Key, "/"); idx > 0 {
			detail.Prefix = (*obj.Key)[:idx+1]
		} else {
			detail.Prefix = "/" // root-level object
		}

		objects = append(objects, detail)
	}
	return objects, nil
}

// gatherBucketStats enumerates objects in a bucket for contexture metadata
func (c *Client) gatherBucketStats(ctx context.Context, bucketName string) (int, int64, []string, []string) {
	maxKeys := int32(1000)
	listOutput, err := c.s3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucketName),
		MaxKeys: &maxKeys,
	})
	if err != nil {
		c.logger.WithField("bucket", bucketName).WithError(err).Warn("Failed to list objects for context")
		return 0, 0, nil, nil
	}

	var totalSize int64
	formatSet := make(map[string]bool)
	var sampleFiles []string
	count := 0

	for _, obj := range listOutput.Contents {
		count++
		if obj.Size != nil {
			totalSize += *obj.Size
		}
		if obj.Key != nil {
			ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(*obj.Key), "."))
			if ext != "" {
				formatSet[ext] = true
			}
			if len(sampleFiles) < 5 {
				sampleFiles = append(sampleFiles, *obj.Key)
			}
		}
	}

	var formats []string
	for f := range formatSet {
		formats = append(formats, f)
	}

	return count, totalSize, formats, sampleFiles
}

func humanizeBytes(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

