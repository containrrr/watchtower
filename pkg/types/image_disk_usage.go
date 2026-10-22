package types

// ImageDiskUsage holds aggregated Docker image storage usage from GET /system/df.
type ImageDiskUsage struct {
	// TotalSize is the number of bytes used by images.
	TotalSize int64
	// Reclaimable is the number of bytes that unused images could free.
	Reclaimable int64
	// TotalCount is the number of images.
	TotalCount int64
	// ActiveCount is the number of images in use by containers.
	ActiveCount int64
}
