package notifications

import (
	"encoding/json"
	"fmt"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// porcelainContainer represents a single container in the porcelain JSON report.
type porcelainContainer struct {
	Name                 string `json:"name"`
	Image                string `json:"image"`
	ImageID              string `json:"image_id"`
	LatestImageID        string `json:"latest_image_id"`
	State                string `json:"state"`
	UpdateAvailable      bool   `json:"update_available"`
	Error                string `json:"error,omitempty"`
	GitRepo              string `json:"git_repo,omitempty"`
	GitRef               string `json:"git_ref,omitempty"`
	Changelog            string `json:"changelog,omitempty"`
	OCISource            string `json:"oci_source,omitempty"`
	ImageURL             string `json:"image_url,omitempty"`
	Documentation        string `json:"documentation,omitempty"`
	CurrentImageVersion  string `json:"current_image_version,omitempty"`
	LatestImageVersion   string `json:"latest_image_version,omitempty"`
	CurrentImageRevision string `json:"current_revision,omitempty"`
	LatestImageRevision  string `json:"latest_revision,omitempty"`
}

// porcelainReport is the top-level JSON structure for --porcelain json.
type porcelainReport struct {
	Containers []porcelainContainer `json:"containers"`
}

// toPorcelainReport converts a types.Report into a porcelainReport.
//
// Parameters:
//   - sourceReport: Source report.
//
// Returns:
//   - porcelainReport: JSON-ready report.
func toPorcelainReport(sourceReport types.Report) porcelainReport {
	if sourceReport == nil {
		return porcelainReport{Containers: make([]porcelainContainer, 0)}
	}

	allContainers := sourceReport.All()
	report := porcelainReport{
		Containers: make([]porcelainContainer, 0, len(allContainers)),
	}

	for _, containerReport := range allContainers {
		container := porcelainContainer{
			Name:                 containerReport.Name(),
			Image:                containerReport.ImageName(),
			ImageID:              containerReport.CurrentImageID().ShortID(),
			LatestImageID:        containerReport.LatestImageID().ShortID(),
			State:                containerReport.State(),
			UpdateAvailable:      containerReport.CurrentImageID() != containerReport.LatestImageID(),
			GitRepo:              containerReport.GitRepo(),
			GitRef:               containerReport.GitRef(),
			Changelog:            containerReport.Changelog(),
			OCISource:            containerReport.Source(),
			ImageURL:             containerReport.ImageURL(),
			Documentation:        containerReport.Documentation(),
			CurrentImageVersion:  containerReport.CurrentImageVersion(),
			LatestImageVersion:   containerReport.LatestImageVersion(),
			CurrentImageRevision: containerReport.CurrentImageRevision(),
			LatestImageRevision:  containerReport.LatestImageRevision(),
		}
		if err := containerReport.Error(); err != "" {
			container.Error = err
		}

		report.Containers = append(report.Containers, container)
	}

	return report
}

// toPorcelainJSON marshals a types.Report to an indented JSON string for templates.
//
// Parameters:
//   - sourceReport: Source report.
//
// Returns:
//   - string: Indented JSON or error string if marshaling fails.
func toPorcelainJSON(sourceReport types.Report) string {
	report := toPorcelainReport(sourceReport)

	bytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Sprintf("failed to marshal porcelain JSON: %v", err)
	}

	return string(bytes)
}
