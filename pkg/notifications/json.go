package notifications

import (
	"encoding/json"

	t "github.com/containrrr/watchtower/pkg/types"
)

type jsonMap = map[string]interface{}

// MarshalJSON implements json.Marshaler
func (d Data) MarshalJSON() ([]byte, error) {
	var entries = make([]jsonMap, len(d.Entries))
	for i, entry := range d.Entries {
		entries[i] = jsonMap{
			`level`:   entry.Level,
			`message`: entry.Message,
			`data`:    entry.Data,
			`time`:    entry.Time,
		}
	}

	var report jsonMap
	if d.Report != nil {
		report = jsonMap{
			`scanned`: marshalReports(d.Report.Scanned()),
			`updated`: marshalReports(d.Report.Updated()),
			`failed`:  marshalReports(d.Report.Failed()),
			`skipped`: marshalReports(d.Report.Skipped()),
			`stale`:   marshalReports(d.Report.Stale()),
			`fresh`:   marshalReports(d.Report.Fresh()),
		}
	}

	return json.Marshal(jsonMap{
		`report`:  report,
		`title`:   d.Title,
		`host`:    d.Host,
		`entries`: entries,
	})
}

func marshalReports(reports []t.ContainerReport) []jsonMap {
	jsonReports := make([]jsonMap, len(reports))
	for i, report := range reports {
		jsonReports[i] = jsonMap{
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
