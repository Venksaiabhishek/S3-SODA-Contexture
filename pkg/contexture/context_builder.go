package contexture

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
)

// ContextBuilder builds OCS-aligned context from MinIO data
type ContextBuilder struct {
	s3Client *s3.Client
	endpoint string
	logger   *logging.Logger
}

// NewContextBuilder creates a new context builder connected to MinIO
func NewContextBuilder(s3Client *s3.Client, endpoint string, logger *logging.Logger) *ContextBuilder {
	return &ContextBuilder{
		s3Client: s3Client,
		endpoint: endpoint,
		logger:   logger,
	}
}

// BuildDataLandscape scans all buckets and builds a full data landscape context
func (cb *ContextBuilder) BuildDataLandscape(ctx context.Context) (*DataLandscape, error) {
	cb.logger.Info("Building data landscape context")

	bucketsOutput, err := cb.s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	landscape := &DataLandscape{
		Endpoint:    cb.endpoint,
		GeneratedAt: time.Now(),
	}

	for _, bucket := range bucketsOutput.Buckets {
		if bucket.Name == nil {
			continue
		}

		bucketCtx, err := cb.BuildBucketContext(ctx, *bucket.Name, bucket.CreationDate)
		if err != nil {
			cb.logger.WithField("bucket", *bucket.Name).WithError(err).Warn("Failed to build bucket context, skipping")
			continue
		}

		landscape.Buckets = append(landscape.Buckets, *bucketCtx)
		landscape.TotalObjects += bucketCtx.ObjectCount
		landscape.TotalSize += bucketCtx.TotalSizeBytes
	}

	landscape.TotalBuckets = len(landscape.Buckets)
	landscape.SizeHuman = humanizeBytes(landscape.TotalSize)

	cb.logger.WithFields(map[string]interface{}{
		"totalBuckets": landscape.TotalBuckets,
		"totalObjects": landscape.TotalObjects,
		"totalSize":    landscape.SizeHuman,
	}).Info("Data landscape context built successfully")

	return landscape, nil
}

// BuildBucketContext builds OCS context for a single bucket
func (cb *ContextBuilder) BuildBucketContext(ctx context.Context, bucketName string, creationDate *time.Time) (*DataSourceContext, error) {
	cb.logger.WithField("bucket", bucketName).Info("Building bucket context")

	dsCtx := &DataSourceContext{
		SourceID:   bucketName,
		SourceType: "s3_bucket",
		SourceName: bucketName,
		Endpoint:   cb.endpoint,
		Tags:       make(map[string]string),
	}

	if creationDate != nil {
		dsCtx.CreatedAt = *creationDate
	}

	// List objects to gather statistics and sample content
	objects, err := cb.listAllObjects(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects in bucket %s: %w", bucketName, err)
	}

	dsCtx.ObjectCount = int64(len(objects))

	formatCounts := make(map[string]int)
	contentTypes := make(map[string]bool)
	var latestModified time.Time

	for _, obj := range objects {
		if obj.Size != nil {
			dsCtx.TotalSizeBytes += *obj.Size
		}
		if obj.LastModified != nil && obj.LastModified.After(latestModified) {
			latestModified = *obj.LastModified
		}

		// Infer format from key extension
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(*obj.Key), "."))
		if ext != "" {
			formatCounts[ext]++
		}
	}

	if !latestModified.IsZero() {
		dsCtx.LastModified = latestModified
	}

	dsCtx.TotalSizeHuman = humanizeBytes(dsCtx.TotalSizeBytes)

	// Collect data formats
	for format := range formatCounts {
		dsCtx.DataFormats = append(dsCtx.DataFormats, format)
	}

	// Collect content types
	for ct := range contentTypes {
		dsCtx.ContentTypes = append(dsCtx.ContentTypes, ct)
	}

	// Sample up to 5 objects for detailed context
	sampleCount := 5
	if len(objects) < sampleCount {
		sampleCount = len(objects)
	}

	for i := 0; i < sampleCount; i++ {
		obj := objects[i]
		objCtx := ObjectContext{
			Key:    *obj.Key,
			Format: strings.ToLower(strings.TrimPrefix(filepath.Ext(*obj.Key), ".")),
		}
		if obj.Size != nil {
			objCtx.Size = *obj.Size
			objCtx.SizeHuman = humanizeBytes(*obj.Size)
		}
		if obj.LastModified != nil {
			objCtx.LastModified = *obj.LastModified
		}
		if obj.ETag != nil {
			objCtx.ETag = *obj.ETag
		}

		dsCtx.SampleObjects = append(dsCtx.SampleObjects, objCtx)
	}

	// Try to infer schemas from sample CSV/JSON files
	for i := 0; i < sampleCount; i++ {
		obj := objects[i]
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(*obj.Key), "."))

		if ext == "csv" || ext == "json" {
			schema, err := cb.inferSchema(ctx, bucketName, *obj.Key, ext)
			if err == nil && schema != nil {
				dsCtx.Schemas = append(dsCtx.Schemas, *schema)
			}
		}
	}

	return dsCtx, nil
}

// GetObjectMetadata retrieves detailed metadata for a specific object
func (cb *ContextBuilder) GetObjectMetadata(ctx context.Context, bucketName, key string) (*ObjectContext, error) {
	headOutput, err := cb.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &bucketName,
		Key:    &key,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}

	objCtx := &ObjectContext{
		Key:      key,
		Format:   strings.ToLower(strings.TrimPrefix(filepath.Ext(key), ".")),
		Metadata: headOutput.Metadata,
	}

	if headOutput.ContentLength != nil {
		objCtx.Size = *headOutput.ContentLength
		objCtx.SizeHuman = humanizeBytes(*headOutput.ContentLength)
	}
	if headOutput.ContentType != nil {
		objCtx.ContentType = *headOutput.ContentType
	}
	if headOutput.LastModified != nil {
		objCtx.LastModified = *headOutput.LastModified
	}
	if headOutput.ETag != nil {
		objCtx.ETag = *headOutput.ETag
	}

	return objCtx, nil
}

// ===== Internal helpers =====

func (cb *ContextBuilder) listAllObjects(ctx context.Context, bucketName string) ([]s3types.Object, error) {
	var allObjects []s3types.Object

	paginator := s3.NewListObjectsV2Paginator(cb.s3Client, &s3.ListObjectsV2Input{
		Bucket:  &bucketName,
		MaxKeys: toInt32(1000),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		allObjects = append(allObjects, page.Contents...)

		// Safety limit: don't scan more than 10,000 objects
		if len(allObjects) > 10000 {
			break
		}
	}

	return allObjects, nil
}

func (cb *ContextBuilder) inferSchema(ctx context.Context, bucketName, key, format string) (*SchemaContext, error) {
	// Only read first 64KB to detect schema
	rangeHeader := "bytes=0-65536"
	getOutput, err := cb.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucketName,
		Key:    &key,
		Range:  &rangeHeader,
	})
	if err != nil {
		return nil, err
	}
	defer getOutput.Body.Close()

	body, err := io.ReadAll(getOutput.Body)
	if err != nil {
		return nil, err
	}

	schema := &SchemaContext{
		Format:     format,
		SampleFile: key,
	}

	switch format {
	case "csv":
		schema.Fields = cb.inferCSVSchema(string(body))
	case "json":
		schema.Fields = cb.inferJSONSchema(body)
	}

	if len(schema.Fields) == 0 {
		return nil, nil // No schema detected
	}

	return schema, nil
}

func (cb *ContextBuilder) inferCSVSchema(data string) []FieldSchema {
	reader := csv.NewReader(strings.NewReader(data))

	// Read header row
	headers, err := reader.Read()
	if err != nil {
		return nil
	}

	// Read first data row for sampling
	dataRow, err := reader.Read()
	if err != nil && err != io.EOF {
		dataRow = nil
	}

	var fields []FieldSchema
	for i, header := range headers {
		field := FieldSchema{
			Name: strings.TrimSpace(header),
			Type: "string", // Default
		}

		if dataRow != nil && i < len(dataRow) {
			field.Sample = dataRow[i]
			field.Type = inferFieldType(dataRow[i])
		}

		fields = append(fields, field)
	}

	return fields
}

func (cb *ContextBuilder) inferJSONSchema(data []byte) []FieldSchema {
	// Try to parse as a JSON object
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err == nil {
		return flattenJSONObject(obj)
	}

	// Try as JSON array — use first element
	var arr []map[string]interface{}
	if err := json.Unmarshal(data, &arr); err == nil && len(arr) > 0 {
		return flattenJSONObject(arr[0])
	}

	return nil
}

func flattenJSONObject(obj map[string]interface{}) []FieldSchema {
	var fields []FieldSchema
	for key, val := range obj {
		field := FieldSchema{
			Name: key,
			Type: inferGoType(val),
		}
		if val != nil {
			field.Sample = fmt.Sprintf("%v", val)
			if len(field.Sample) > 100 {
				field.Sample = field.Sample[:100] + "..."
			}
		} else {
			field.Nullable = true
		}
		fields = append(fields, field)
	}
	return fields
}

func inferFieldType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "string"
	}
	// Check if it's a number
	if _, err := fmt.Sscanf(value, "%f", new(float64)); err == nil {
		if !strings.Contains(value, ".") {
			return "integer"
		}
		return "number"
	}
	// Check for boolean
	lower := strings.ToLower(value)
	if lower == "true" || lower == "false" {
		return "boolean"
	}
	// Check for date-like patterns
	for _, layout := range []string{time.RFC3339, "2006-01-02", "01/02/2006"} {
		if _, err := time.Parse(layout, value); err == nil {
			return "date"
		}
	}
	return "string"
}

func inferGoType(val interface{}) string {
	switch val.(type) {
	case float64:
		return "number"
	case bool:
		return "boolean"
	case string:
		return "string"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	case nil:
		return "null"
	default:
		return "string"
	}
}

func humanizeBytes(bytes int64) string {
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

func toInt32(v int32) *int32 {
	return &v
}
