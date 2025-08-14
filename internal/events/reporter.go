package events

import (
	"context"
	"sync"
	"time"

	"github.com/9seconds/mtg/v2/logger"
	"github.com/9seconds/mtg/v2/stats"
	"github.com/9seconds/mtg/v2/sspanel"
)

type SSPanelReporter struct {
	client           *sspanel.Client
	trafficBatchSize int
	trafficFlushDelay time.Duration

	mu      sync.Mutex
	traffic []stats.TrafficRecord
	closed  bool
}

func NewSSPanelReporter(client *sspanel.Client, trafficBatchSize int, trafficFlushDelay time.Duration) *SSPanelReporter {
	reporter := &SSPanelReporter{
		client:           client,
		trafficBatchSize: trafficBatchSize,
		trafficFlushDelay: trafficFlushDelay,
		traffic:          make([]stats.TrafficRecord, 0, trafficBatchSize),
	}

	go reporter.flushLoop()

	return reporter
}

func (s *SSPanelReporter) flushLoop() {
	ticker := time.NewTicker(s.trafficFlushDelay)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.flush()
		}
	}
}

func (s *SSPanelReporter) flush() {
	s.mu.Lock()
	if len(s.traffic) == 0 {
		s.mu.Unlock()
		return
	}

	records := s.traffic
	s.traffic = make([]stats.TrafficRecord, 0, s.trafficBatchSize)
	s.mu.Unlock()

	s.flushBatch(records)
}

func (s *SSPanelReporter) flushBatch(records []stats.TrafficRecord) {
	reports := make([]sspanel.TrafficReport, 0, len(records))
	
	for _, r := range records {
		reports = append(reports, sspanel.TrafficReport{
			UserID:   r.UserID,
			Upload:   r.Upload,
			Download: r.Download,
		})
	}
	
	// 最多重试3次
	for i := 0; i < 3; i++ {
		if err := s.client.ReportTraffic(context.Background(), reports); err == nil {
			return // 成功
		}
		time.Sleep(time.Duration(i+1) * time.Second) // 指数退避
	}
	
	// 记录最终失败
	logger.Error("Failed to report traffic after 3 attempts")
}

func (s *SSPanelReporter) ReportTraffic(record stats.TrafficRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	s.traffic = append(s.traffic, record)

	if len(s.traffic) >= s.trafficBatchSize {
		go s.flush()
	}
}

func (s *SSPanelReporter) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	s.flush() // 立即上报剩余流量
}

// 实现 events.Observer 接口
func (s *SSPanelReporter) Notify(ctx context.Context, event mtglib.Event) {}