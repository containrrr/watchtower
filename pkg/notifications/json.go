package notifications

import (
	"encoding/json"

	t "github.com/containrrr/watchtower/pkg/types"
)

type JSONMap = map[string]interface{}

// MarshalJSON implements json.Marshaler
func (d Data) MarshalJSON() ([]byte, error) {
	var entries = make([]JSONMap, len(d.Entries))
	for i, entry := range d.Entries {
		entries[i] = JSONMap{
			`level`:   entry.Level,
			`message`: entry.Message,
			`data`:    entry.Data,
			`time`:    entry.Time,
		}
	}

	var report JSONMap
	if d.Report != nil {
		report = JSONMap{
			`scanned`: marshalReports(d.Report.Scanned()),
			`updated`: marshalReports(d.Report.Updated()),
			`failed`:  marshalReports(d.Report.Failed()),
			`skipped`: marshalReports(d.Report.Skipped()),
			`stale`:   marshalReports(d.Report.Stale()),
			`fresh`:   marshalReports(d.Report.Fresh()),
		}
	}

	return json.Marshal(JSONMap{
		`report`:  report,
		`title`:   d.Title,
		`host`:    d.Host,
		`entries`: entries,
	})
}

func marshalReports(reports []t.ContainerReport) []JSONMap {
	jsonReports := make([]JSONMap, len(reports))
	for i, report := range reports {
		jsonReports[i] = JSONMap{
			`id`:             report.ID().ShortID(),
			`name`:           report.Name(),
			`currentImageId`: report.CurrentImageID().ShortID(),
			`latestImageId`:  report.LatestImageID().ShortID(),
			`imageName`:      report.ImageName(),
			`state`:          report.State(),
		}
		if errorMessage := report.Error(); errorMessage != "" {
			jsonReports[i][`error`] = errorMessage
		}
	}
	return jsonReports
}

var _ json.Marshaler = &Data{}

func toJSON(v interface{}) string {
	if bytes, err := json.MarshalIndent(v, "", "  "); err != nil {
		LocalLog.Errorf("failed to marshal JSON in notification template: %v", err)
		return ""
	} else {
		return string(bytes)
	}
}
