package notifications

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

var _ json.Marshaler = &notificationData{}

// Errors for JSON marshaling.
var (
	// errMarshalFailed indicates a failure to marshal notification data to JSON.
	errMarshalFailed = errors.New("failed to marshal notification data")
)

// jsonMap is a type alias for a JSON-compatible map.
type jsonMap = map[string]any

// MarshalJSON implements json.Marshaler for Data.
//
// Returns:
//   - []byte: JSON-encoded data.
//   - error: Non-nil if marshaling fails, nil on success.
func (d notificationData) MarshalJSON() ([]byte, error) {
	// Convert log entries to JSON maps.
	entries := make([]jsonMap, len(d.Entries))
	for i, entry := range d.Entries {
		entries[i] = jsonMap{
			"level":   entry.Level,
			"message": entry.Message,
			"data":    entry.Data,
			"time":    entry.Time,
		}
	}

	// Include report data if present.
	var report jsonMap

	if d.Report != nil {
		report = jsonMap{
			"scanned":   marshalReports(d.Report.Scanned()),
			"updated":   marshalReports(d.Report.Updated()),
			"restarted": marshalReports(d.Report.Restarted()),
			"failed":    marshalReports(d.Report.Failed()),
			"skipped":   marshalReports(d.Report.Skipped()),
			"stale":     marshalReports(d.Report.Stale()),
			"fresh":     marshalReports(d.Report.Fresh()),
		}
	}

	// Build final JSON structure.
	data := jsonMap{
		"report":  report,
		"title":   d.Title,
		"host":    d.Host,
		"entries": entries,
	}

	// Marshal to JSON bytes.
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errMarshalFailed, err)
	}

	return bytes, nil
}

// marshalReports converts ContainerReport slice to JSON-compatible maps.
//
// Parameters:
//   - reports: List of container reports.
//
// Returns:
//   - []jsonMap: JSON maps of report data.
func marshalReports(reports []types.ContainerReport) []jsonMap {
	jsonReports := make([]jsonMap, len(reports))
	for i, report := range reports {
		containerID := report.ID().ShortID()

		// Populate base report fields.
		jsonReports[i] = jsonMap{
			"id":             containerID,
			"name":           report.Name(),
			"currentImageId": report.CurrentImageID().ShortID(),
			"latestImageId":  report.LatestImageID().ShortID(),
			"imageName":      report.ImageName(),
			"state":          report.State(),
		}

		// Add error if present.
		errorMessage := report.Error()
		if errorMessage != "" {
			jsonReports[i]["error"] = errorMessage
		}

		if gitRepo := report.GitRepo(); gitRepo != "" {
			jsonReports[i]["gitRepo"] = gitRepo
		}

		if gitRef := report.GitRef(); gitRef != "" {
			jsonReports[i]["gitRef"] = gitRef
		}

		if changelog := report.Changelog(); changelog != "" {
			jsonReports[i]["changelog"] = changelog
		}

		if source := report.Source(); source != "" {
			jsonReports[i]["ociSource"] = source
		}

		if imageURL := report.ImageURL(); imageURL != "" {
			jsonReports[i]["imageUrl"] = imageURL
		}

		if documentation := report.Documentation(); documentation != "" {
			jsonReports[i]["documentation"] = documentation
		}

		if currentRevision := report.CurrentImageRevision(); currentRevision != "" {
			jsonReports[i]["currentImageRevision"] = currentRevision
		}

		if latestRevision := report.LatestImageRevision(); latestRevision != "" {
			jsonReports[i]["latestImageRevision"] = latestRevision
		}

		if currentVersion := report.CurrentImageVersion(); currentVersion != "" {
			jsonReports[i]["currentImageVersion"] = currentVersion
		}

		if latestVersion := report.LatestImageVersion(); latestVersion != "" {
			jsonReports[i]["latestImageVersion"] = latestVersion
		}
	}

	return jsonReports
}
