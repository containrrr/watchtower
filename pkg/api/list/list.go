package list

import (
	"encoding/json"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/containrrr/watchtower/pkg/container"
	filters "github.com/containrrr/watchtower/pkg/filters"
)

// Handler is an HTTP handle for serving list data
type Handler struct {
	Path   string
	Client container.Client
}

type ContainerListEntry struct {
	ContainerId      string
	ContainerName    string
	ImageName        string
	ImageNameShort   string
	ImageVersion     string
	ImageCreatedDate string
}
type ListResponse struct {
	Containers []ContainerListEntry
}

// New is a factory function creating a new List instance
func New(client container.Client) *Handler {
	return &Handler{
		Path:   "/v1/list",
		Client: client,
	}
}

// HandleGet is the actual http.HandleGet function doing all the heavy lifting
func (handle *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.Info("Calling List API with unsupported method")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	log.Info("List containers triggered by HTTP API request.")

	client := handle.Client
	filter := filters.NoFilter
	containers, err := client.ListContainers(filter)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}

	data := ListResponse{Containers: []ContainerListEntry{}}

	for _, c := range containers {
		data.Containers = append(data.Containers, ContainerListEntry{
			ContainerId:      c.ID().ShortID(),
			ContainerName:    c.Name()[1:],
			ImageName:        c.ImageName(),
			ImageNameShort:   strings.Split(c.ImageName(), ":")[0],
			ImageCreatedDate: c.ImageInfo().Created,
			ImageVersion:     c.ImageID().ShortID(),
		})
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("Error happened in JSON marshal. Err: %s", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)
}
