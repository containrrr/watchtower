package updates

import (
	. "github.com/containrrr/watchtower/pkg/api/prelude"
	"github.com/containrrr/watchtower/pkg/filters"
	"github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
	"sync"
)

func postV2(onInvoke InvokedFunc, updateLock *sync.Mutex, monitorOnly bool) HandlerFunc {
	return func(c *Context) Response {
		log.Info("Updates triggered by HTTP API request.")

		images := parseImages(c.Request.URL)

		if updateLock.TryLock() {
			defer updateLock.Unlock()

			result := onInvoke(func(up *types.UpdateParams) {
				up.Filter = filters.FilterByImage(images, up.Filter)
				up.MonitorOnly = monitorOnly
			})
			return OK(result)
		} else {
			return Error(ErrUpdateRunning)
		}
	}
}

func PostV2Check(onInvoke InvokedFunc, updateLock *sync.Mutex) HandlerFunc {
	return postV2(onInvoke, updateLock, true)
}

func PostV2Apply(onInvoke InvokedFunc, updateLock *sync.Mutex) HandlerFunc {
	return postV2(onInvoke, updateLock, false)
}
