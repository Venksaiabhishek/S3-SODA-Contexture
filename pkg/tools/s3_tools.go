package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
)

// sanitizeBucketName normalizes a user-provided bucket name to conform to S3/MinIO naming rules:
// - Lowercase only
// - Only a-z, 0-9, hyphens allowed
// - Must be 3-63 characters
// - Cannot start or end with a hyphen
func sanitizeBucketName(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("bucket name cannot be empty")
	}

	// Lowercase
	sanitized := strings.ToLower(name)

	// Replace underscores and spaces with hyphens
	sanitized = strings.ReplaceAll(sanitized, "_", "-")
	sanitized = strings.ReplaceAll(sanitized, " ", "-")

	// Strip all characters that are not a-z, 0-9, or hyphen
	re := regexp.MustCompile(`[^a-z0-9-]`)
	sanitized = re.ReplaceAllString(sanitized, "")

	// Collapse multiple consecutive hyphens
	reMultiHyphen := regexp.MustCompile(`-{2,}`)
	sanitized = reMultiHyphen.ReplaceAllString(sanitized, "-")

	// Trim leading/trailing hyphens
	sanitized = strings.Trim(sanitized, "-")

	if len(sanitized) < 3 {
		return "", fmt.Errorf("bucket name '%s' is too short after sanitization (minimum 3 characters)", name)
	}
	if len(sanitized) > 63 {
		sanitized = sanitized[:63]
	}

	return sanitized, nil
}

// ===== CreateS3BucketTool =====

type CreateS3BucketTool struct {
	*BaseTool
	awsClient *aws.Client
}

func NewCreateS3BucketTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"bucketName": map[string]interface{}{
				"type":        "string",
				"description": "The name of the new S3 bucket to create. Will be auto-normalized to lowercase.",
			},
		},
		"required": []interface{}{"bucketName"},
	}

	baseTool := NewBaseTool(
		"create-s3-bucket",
		"Create a new S3 bucket in MinIO. Bucket names are automatically normalized to lowercase.",
		"s3",
		actionType,
		inputSchema,
		logger,
	)

	return &CreateS3BucketTool{
		BaseTool:  baseTool,
		awsClient: awsClient,
	}
}

func (t *CreateS3BucketTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	rawName, ok := arguments["bucketName"].(string)
	if !ok || rawName == "" {
		return t.CreateErrorResponse("bucketName is required")
	}

	bucketName, err := sanitizeBucketName(rawName)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Invalid bucket name: %s", err.Error()))
	}

	resource, err := t.awsClient.CreateS3Bucket(ctx, bucketName)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create S3 bucket: %s", err.Error()))
	}

	message := fmt.Sprintf("Successfully created bucket '%s'", resource.ID)
	if rawName != bucketName {
		message = fmt.Sprintf("Successfully created bucket '%s' (normalized from '%s')", resource.ID, rawName)
	}

	data := map[string]interface{}{
		"bucketName": resource.ID,
		"state":      "available",
	}

	return t.CreateSuccessResponse(message, data)
}

// ===== ListS3BucketsTool =====

type ListS3BucketsTool struct {
	*BaseTool
	awsClient *aws.Client
}

func NewListS3BucketsTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	baseTool := NewBaseTool(
		"list-s3-buckets",
		"List all S3 buckets in MinIO with full SODA Contexture details: creation date, object count, total size, data formats, and sample files. Use this tool when the user asks about buckets, what data is stored, or wants a data landscape overview.",
		"s3",
		actionType,
		inputSchema,
		logger,
	)

	return &ListS3BucketsTool{
		BaseTool:  baseTool,
		awsClient: awsClient,
	}
}

func (t *ListS3BucketsTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	bucketContexts, err := t.awsClient.ListS3BucketsWithContext(ctx)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to list S3 buckets: %s", err.Error()))
	}

	if len(bucketContexts) == 0 {
		return t.CreateSuccessResponse("No S3 buckets found in MinIO. The storage is empty.", map[string]interface{}{
			"count":   0,
			"buckets": []interface{}{},
		})
	}

	// Build rich context details per bucket
	var summaryParts []string
	bucketDetails := make([]map[string]interface{}, 0, len(bucketContexts))

	for _, bc := range bucketContexts {
		detail := map[string]interface{}{
			"bucketName":  bc.Name,
			"createdAt":   bc.CreatedAt,
			"objectCount": bc.ObjectCount,
			"totalSize":   bc.SizeHuman,
			"dataFormats": bc.DataFormats,
			"sampleFiles": bc.SampleFiles,
		}
		bucketDetails = append(bucketDetails, detail)

		// Build human-readable summary per bucket
		summary := fmt.Sprintf("• %s — Created: %s, Objects: %d, Size: %s",
			bc.Name, bc.CreatedAt, bc.ObjectCount, bc.SizeHuman)
		if len(bc.DataFormats) > 0 {
			summary += fmt.Sprintf(", Formats: [%s]", strings.Join(bc.DataFormats, ", "))
		}
		if len(bc.SampleFiles) > 0 {
			summary += fmt.Sprintf(", Sample files: [%s]", strings.Join(bc.SampleFiles, ", "))
		}
		if bc.ObjectCount == 0 {
			summary += " (empty bucket)"
		}
		summaryParts = append(summaryParts, summary)
	}

	message := fmt.Sprintf("=== SODA Contexture: MinIO Data Landscape ===\n\nFound %d bucket(s):\n\n%s",
		len(bucketContexts), strings.Join(summaryParts, "\n"))

	data := map[string]interface{}{
		"count":       len(bucketContexts),
		"bucketNames": func() []string {
			var names []string
			for _, bc := range bucketContexts {
				names = append(names, bc.Name)
			}
			return names
		}(),
		"buckets": bucketDetails,
	}

	return t.CreateSuccessResponse(message, data)
}

// ===== DeleteS3BucketTool =====

type DeleteS3BucketTool struct {
	*BaseTool
	awsClient *aws.Client
}

func NewDeleteS3BucketTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"bucketName": map[string]interface{}{
				"type":        "string",
				"description": "The name of the S3 bucket to delete. Will be auto-normalized to lowercase.",
			},
		},
		"required": []interface{}{"bucketName"},
	}

	baseTool := NewBaseTool(
		"delete-s3-bucket",
		"Delete an S3 bucket in MinIO. Bucket names are automatically normalized to lowercase.",
		"s3",
		actionType,
		inputSchema,
		logger,
	)

	return &DeleteS3BucketTool{
		BaseTool:  baseTool,
		awsClient: awsClient,
	}
}

func (t *DeleteS3BucketTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	rawName, ok := arguments["bucketName"].(string)
	if !ok || rawName == "" {
		return t.CreateErrorResponse("bucketName is required")
	}

	bucketName, err := sanitizeBucketName(rawName)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Invalid bucket name: %s", err.Error()))
	}

	err = t.awsClient.DeleteS3Bucket(ctx, bucketName)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to delete S3 bucket: %s", err.Error()))
	}

	message := fmt.Sprintf("Successfully deleted bucket '%s'", bucketName)
	data := map[string]interface{}{
		"bucketName": bucketName,
		"status":     "deleted",
	}

	return t.CreateSuccessResponse(message, data)
}

// ===== ListS3ObjectsTool =====

type ListS3ObjectsTool struct {
	*BaseTool
	awsClient *aws.Client
}

func NewListS3ObjectsTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"bucketName": map[string]interface{}{
				"type":        "string",
				"description": "The name of the S3 bucket to list objects from.",
			},
		},
		"required": []interface{}{"bucketName"},
	}

	baseTool := NewBaseTool(
		"list-s3-objects",
		"List all objects in a specific S3 bucket WITH FULL METADATA: key, size, sizeHuman, lastModified timestamp, etag, and prefix/namespace. Use this tool for: object-level queries, finding largest/smallest objects, checking last modified dates, correlating objects by prefix or timestamp.",
		"s3",
		actionType,
		inputSchema,
		logger,
	)

	return &ListS3ObjectsTool{
		BaseTool:  baseTool,
		awsClient: awsClient,
	}
}

func (t *ListS3ObjectsTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	rawName, ok := arguments["bucketName"].(string)
	if !ok || rawName == "" {
		return t.CreateErrorResponse("bucketName is required")
	}

	bucketName, err := sanitizeBucketName(rawName)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Invalid bucket name: %s", err.Error()))
	}

	objects, err := t.awsClient.ListS3ObjectsDetailed(ctx, bucketName)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to list objects in bucket '%s': %s", bucketName, err.Error()))
	}

	if len(objects) == 0 {
		return t.CreateSuccessResponse(fmt.Sprintf("Bucket '%s' is empty (0 objects).", bucketName), map[string]interface{}{
			"bucketName": bucketName,
			"count":      0,
			"objects":    []interface{}{},
		})
	}

	// Build rich human-readable summary with full metadata per object
	var summaryParts []string
	objectDetails := make([]map[string]interface{}, 0, len(objects))

	for _, obj := range objects {
		summary := fmt.Sprintf("• %s — Size: %s (%d bytes), LastModified: %s, Prefix: %s",
			obj.Key, obj.SizeHuman, obj.Size, obj.LastModified, obj.Prefix)
		summaryParts = append(summaryParts, summary)

		objectDetails = append(objectDetails, map[string]interface{}{
			"key":          obj.Key,
			"size":         obj.Size,
			"sizeHuman":    obj.SizeHuman,
			"lastModified": obj.LastModified,
			"etag":         obj.ETag,
			"prefix":       obj.Prefix,
		})
	}

	message := fmt.Sprintf("=== Objects in bucket '%s' (%d total) ===\n\n%s",
		bucketName, len(objects), strings.Join(summaryParts, "\n"))

	data := map[string]interface{}{
		"bucketName": bucketName,
		"count":      len(objects),
		"objects":    objectDetails,
	}

	return t.CreateSuccessResponse(message, data)
}
