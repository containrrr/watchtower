package notifications

import (
	t "github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
)

// StaticData is the part of the notification template data model set upon initialization
type StaticData struct {
	Title string
	Host  string
}

// Data is the notification template data model
type Data struct {
	StaticData
	Entries []*log.Entry
	Report  t.Report
}
