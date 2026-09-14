package health

import (
	"net/http"
	"sync/atomic"
	"time"
)

type Tracker struct {
	last atomic.Int64
}

func NewTracker() *Tracker {
	t := &Tracker{}
	t.MarkHealthy()
	return t
}

func (t *Tracker) MarkHealthy() {
	t.last.Store(time.Now().UnixNano())
}

func (t *Tracker) Handler(maxAge time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		last := time.Unix(0, t.last.Load())

		if time.Since(last) > maxAge {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
