package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/contexture"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
)

// ===== AnalyzeDataLandscapeTool =====

type AnalyzeDataLandscapeTool struct {
	*BaseTool
	ctxBuilder *contexture.ContextBuilder
}

func NewAnalyzeDataLandscapeTool(ctxBuilder *contexture.ContextBuilder, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	baseTool := NewBaseTool(
		"analyze-data-landscape",
		"Scan all MinIO buckets and build a comprehensive data landscape overview following the Open Context Specification (OCS). Returns bucket inventories, object counts, data formats, and detected schemas.",
		"contexture",
		actionType,
		inputSchema,
		logger,
	)

	return &AnalyzeDataLandscapeTool{
		BaseTool:   baseTool,
		ctxBuilder: ctxBuilder,
	}
}

func (t *AnalyzeDataLandscapeTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	landscape, err := t.ctxBuilder.BuildDataLandscape(ctx)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to analyze data landscape: %s", err.Error()))
	}

	landscapeJSON, _ := json.MarshalIndent(landscape, "", "  ")

	message := fmt.Sprintf("Data landscape analysis complete: %d buckets, %d total objects, %s total size",
		landscape.TotalBuckets, landscape.TotalObjects, landscape.SizeHuman)

	data := map[string]interface{}{
		"totalBuckets": landscape.TotalBuckets,
		"totalObjects": landscape.TotalObjects,
		"totalSize":    landscape.SizeHuman,
		"landscape":    string(landscapeJSON),
	}

	return t.CreateSuccessResponse(message, data)
}

// ===== DescribeBucketContextTool =====

type DescribeBucketContextTool struct {
	*BaseTool
	ctxBuilder *contexture.ContextBuilder
}

func NewDescribeBucketContextTool(ctxBuilder *contexture.ContextBuilder, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"bucketName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the bucket to describe",
			},
		},
		"required": []interface{}{"bucketName"},
	}

	baseTool := NewBaseTool(
		"describe-bucket-context",
		"Build a full OCS-aligned context for a specific MinIO bucket, including object inventory, data formats, and inferred schemas from CSV/JSON files.",
		"contexture",
		actionType,
		inputSchema,
		logger,
	)

	return &DescribeBucketContextTool{
		BaseTool:   baseTool,
		ctxBuilder: ctxBuilder,
	}
}

func (t *DescribeBucketContextTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	bucketName, ok := arguments["bucketName"].(string)
	if !ok || bucketName == "" {
		return t.CreateErrorResponse("bucketName is required")
	}

	// Normalize bucket name
	sanitized, err := sanitizeBucketName(bucketName)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Invalid bucket name: %s", err.Error()))
	}

	bucketCtx, err := t.ctxBuilder.BuildBucketContext(ctx, sanitized, nil)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to describe bucket context: %s", err.Error()))
	}

	contextJSON, _ := json.MarshalIndent(bucketCtx, "", "  ")

	message := fmt.Sprintf("Bucket '%s': %d objects, %s, formats: %v",
		sanitized, bucketCtx.ObjectCount, bucketCtx.TotalSizeHuman, bucketCtx.DataFormats)

	data := map[string]interface{}{
		"bucketName":   sanitized,
		"objectCount":  bucketCtx.ObjectCount,
		"totalSize":    bucketCtx.TotalSizeHuman,
		"dataFormats":  bucketCtx.DataFormats,
		"schemaCount":  len(bucketCtx.Schemas),
		"bucketContext": string(contextJSON),
	}

	return t.CreateSuccessResponse(message, data)
}

// ===== GetObjectMetadataTool =====

type GetObjectMetadataTool struct {
	*BaseTool
	ctxBuilder *contexture.ContextBuilder
}

func NewGetObjectMetadataTool(ctxBuilder *contexture.ContextBuilder, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"bucketName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the bucket containing the object",
			},
			"objectKey": map[string]interface{}{
				"type":        "string",
				"description": "Key (path) of the object within the bucket",
			},
		},
		"required": []interface{}{"bucketName", "objectKey"},
	}

	baseTool := NewBaseTool(
		"get-object-metadata",
		"Get detailed metadata for a specific object in a MinIO bucket, including content type, size, and custom metadata.",
		"contexture",
		actionType,
		inputSchema,
		logger,
	)

	return &GetObjectMetadataTool{
		BaseTool:   baseTool,
		ctxBuilder: ctxBuilder,
	}
}

func (t *GetObjectMetadataTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	bucketName, ok := arguments["bucketName"].(string)
	if !ok || bucketName == "" {
		return t.CreateErrorResponse("bucketName is required")
	}
	objectKey, ok := arguments["objectKey"].(string)
	if !ok || objectKey == "" {
		return t.CreateErrorResponse("objectKey is required")
	}

	sanitized, err := sanitizeBucketName(bucketName)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Invalid bucket name: %s", err.Error()))
	}

	objCtx, err := t.ctxBuilder.GetObjectMetadata(ctx, sanitized, objectKey)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to get object metadata: %s", err.Error()))
	}

	metadataJSON, _ := json.MarshalIndent(objCtx, "", "  ")

	message := fmt.Sprintf("Object '%s' in bucket '%s': %s, format: %s, content-type: %s",
		objectKey, sanitized, objCtx.SizeHuman, objCtx.Format, objCtx.ContentType)

	data := map[string]interface{}{
		"bucketName":  sanitized,
		"objectKey":   objectKey,
		"size":        objCtx.SizeHuman,
		"format":      objCtx.Format,
		"contentType": objCtx.ContentType,
		"metadata":    string(metadataJSON),
	}

	return t.CreateSuccessResponse(message, data)
}
