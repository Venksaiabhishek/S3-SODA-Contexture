package contexture

import "time"

// ===== Open Context Specification (OCS) Types =====
// These types define how data sources in MinIO are described in a structured,
// standardized way following the SODA Contexture Open Context Specification.

// DataSourceContext describes a MinIO bucket as a data source with full metadata.
type DataSourceContext struct {
	// Source identification
	SourceID    string `json:"sourceId"`
	SourceType  string `json:"sourceType"`  // "s3_bucket"
	SourceName  string `json:"sourceName"`
	Endpoint    string `json:"endpoint"`

	// Data landscape metadata
	ObjectCount    int64     `json:"objectCount"`
	TotalSizeBytes int64     `json:"totalSizeBytes"`
	TotalSizeHuman string    `json:"totalSizeHuman"`
	CreatedAt      time.Time `json:"createdAt,omitempty"`
	LastModified   time.Time `json:"lastModified,omitempty"`

	// Content analysis
	DataFormats   []string         `json:"dataFormats,omitempty"`   // e.g. ["csv", "json", "parquet"]
	ContentTypes  []string         `json:"contentTypes,omitempty"` // MIME types found
	SampleObjects []ObjectContext  `json:"sampleObjects,omitempty"`

	// Schema information (inferred)
	Schemas []SchemaContext `json:"schemas,omitempty"`

	// Annotations
	Tags   map[string]string `json:"tags,omitempty"`
	Labels []string          `json:"labels,omitempty"`
}

// ObjectContext describes a single object/file within a bucket.
type ObjectContext struct {
	Key          string            `json:"key"`
	Size         int64             `json:"size"`
	SizeHuman    string            `json:"sizeHuman"`
	ContentType  string            `json:"contentType,omitempty"`
	LastModified time.Time         `json:"lastModified,omitempty"`
	ETag         string            `json:"etag,omitempty"`
	Format       string            `json:"format,omitempty"` // inferred: csv, json, parquet, etc.
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// SchemaContext describes the inferred schema of data within a bucket.
type SchemaContext struct {
	Format     string        `json:"format"`     // csv, json, parquet
	SampleFile string        `json:"sampleFile"` // which file was sampled
	Fields     []FieldSchema `json:"fields,omitempty"`
	RowCount   int64         `json:"rowCount,omitempty"` // estimated
}

// FieldSchema describes a single field in a detected schema.
type FieldSchema struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // string, number, boolean, date, etc.
	Nullable bool   `json:"nullable,omitempty"`
	Sample   string `json:"sample,omitempty"` // sample value
}

// DataLandscape is the top-level context for the entire MinIO data landscape.
type DataLandscape struct {
	Endpoint     string              `json:"endpoint"`
	TotalBuckets int                 `json:"totalBuckets"`
	TotalObjects int64               `json:"totalObjects"`
	TotalSize    int64               `json:"totalSizeBytes"`
	SizeHuman    string              `json:"totalSizeHuman"`
	Buckets      []DataSourceContext `json:"buckets"`
	GeneratedAt  time.Time           `json:"generatedAt"`
}

// RelationshipContext describes cross-bucket data relationships.
type RelationshipContext struct {
	SourceBucket string `json:"sourceBucket"`
	TargetBucket string `json:"targetBucket"`
	Type         string `json:"type"`        // "derived_from", "feeds_into", "shared_schema"
	Description  string `json:"description"`
}
