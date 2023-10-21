package notifications

import (
	"encoding/json"
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

	return json.Marshal(jsonMap{
		`report`:  d.Report,
		`title`:   d.Title,
		`host`:    d.Host,
		`entries`: entries,
	})
}

var _ json.Marshaler = &Data{}
