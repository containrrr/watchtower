package types

// RemovedImageInfo represents information about an image that was cleaned up during update operations.
// It tracks the image ID, container ID, image name, and the container that was using the old image before cleanup.
type RemovedImageInfo struct {
	// ImageID is the ID of the image that was cleaned up.
	ImageID ImageID `json:"image_id"`
	// ContainerID is the ID of the container that was using this image.
	ContainerID ContainerID `json:"container_id"`
	// ImageName is the name/tag of the image that was cleaned up.
	ImageName string `json:"image_name"`
	// ContainerName is the name of the container that was using this image before the update.
	ContainerName string `json:"container_name"`
}
