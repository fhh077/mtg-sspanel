package stats

import (
	"sync/atomic"
	"time"
)

type TrafficRecord struct {
	UserID   string
	Start    time.Time
	End      time.Time
	Upload   int64
	Download int64
}

type TrafficRecorder struct {
	userID string
	start  time.Time
	up     int64
	down   int64
}

func NewTrafficRecorder(userID string) *TrafficRecorder {
	return &TrafficRecorder{
		userID: userID,
		start:  time.Now(),
	}
}

func (t *TrafficRecorder) RecordUpload(n int) {
	atomic.AddInt64(&t.up, int64(n))
}

func (t *TrafficRecorder) RecordDownload(n int) {
	atomic.AddInt64(&t.down, int64(n))
}

func (t *TrafficRecorder) Finalize() TrafficRecord {
	return TrafficRecord{
		UserID:   t.userID,
		Start:    t.start,
		End:      time.Now(),
		Upload:   atomic.LoadInt64(&t.up),
		Download: atomic.LoadInt64(&t.down),
	}
}