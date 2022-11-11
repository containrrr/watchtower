package check

import (
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/types"
)

// Handler is an HTTP handle for serving list data
type Handler struct {
	Path   string
	Client container.Client
}

// CheckRequest defines the type for the request data of the Check endpoint
type CheckRequest struct {
	ContainerId string
}

// CheckResponse defines the type for the response data of the Check endpoint
type CheckResponse struct {
	ContainerId       string
	HasUpdate         bool
	NewVersion        string
	NewVersionCreated string
}

// New is a factory function creating a new List instance
func New(client container.Client) *Handler {
	return &Handler{
		Path:   "/v1/check",
		Client: client,
	}
}

// HandlePost is the actual http.HandlePost function doing all the heavy lifting
func (handle *Handler) HandlePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Info("Calling Check API with unsupported method")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	log.Info("Check for update triggered by HTTP API request.")

	var request CheckRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	client := handle.Client
	container, err := client.GetContainer(types.ContainerID(request.ContainerId))
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	stale, newestImage, created, err := client.IsContainerStale(container)

	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	data := CheckResponse{
		ContainerId:       request.ContainerId,
		HasUpdate:         stale,
		NewVersion:        newestImage.ShortID(),
		NewVersionCreated: created,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("Error happened in JSON marshal. Err: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)
}
