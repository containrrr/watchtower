package updates

import (
	. "github.com/containrrr/watchtower/pkg/api/prelude"
	"github.com/containrrr/watchtower/pkg/filters"
	"github.com/containrrr/watchtower/pkg/types"
	"sync"

	log "github.com/sirupsen/logrus"
)

// PostV1 creates an API http.HandlerFunc for V1 of updates
func PostV1(updateFn InvokedFunc, updateLock *sync.Mutex) HandlerFunc {
	return func(c *Context) Response {
		log.Info("Updates triggered by HTTP API request.")

		images := parseImages(c.Request.URL)

		if !updateLock.TryLock() {
			if len(images) > 0 {
				// If images have been passed, wait until the current updates are done
				updateLock.Lock()
			} else {
				// If a full update is running (no explicit image filter), skip this update
				log.Debug("Skipped. Another updates already running.")
				return OK(nil) // For backwards compatibility
			}
		}

		defer updateLock.Unlock()
		_ = updateFn(func(up *types.UpdateParams) {
			up.Filter = filters.FilterByImage(images, up.Filter)
		})

		return OK(nil)
	}
}
