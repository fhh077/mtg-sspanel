package mtglib

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/9seconds/mtg/v2/essentials"
	"github.com/9seconds/mtg/v2/mtglib/internal/faketls"
	"github.com/9seconds/mtg/v2/mtglib/internal/faketls/record"
	"github.com/9seconds/mtg/v2/mtglib/internal/obfuscated2"
	"github.com/9seconds/mtg/v2/mtglib/internal/relay"
	"github.com/9seconds/mtg/v2/mtglib/internal/telegram"
	"github.com/9seconds/mtg/v2/stats" // 新增
	"github.com/panjf2000/ants/v2"
)

// Proxy is an MTPROTO proxy structure.
type Proxy struct {
	ctx             context.Context
	ctxCancel       context.CancelFunc
	streamWaitGroup sync.WaitGroup

	allowFallbackOnUnknownDC bool
	tolerateTimeSkewness     time.Duration
	domainFrontingPort       int
	workerPool               *ants.PoolWithFunc
	telegram                 *telegram.Telegram

	secret          Secret
	network         Network
	antiReplayCache AntiReplayCache
	blocklist       IPBlocklist
	allowlist       IPBlocklist
	eventStream     EventStream
	logger          Logger
}

type streamContext struct {
	ctx          context.Context
	ctxCancel    context.CancelFunc
	clientConn   essentials.Conn
	telegramConn essentials.Conn
	streamID     string
	dc           int
	logger       Logger
	
	// 新增字段
	userID          string             // 用户ID
	trafficRecorder *stats.TrafficRecorder // 流量记录器
}

// 新增方法：记录上传流量
func (s *streamContext) RecordUpload(n int) {
	if s.trafficRecorder != nil {
		s.trafficRecorder.RecordUpload(n)
	}
}

// 新增方法：记录下载流量
func (s *streamContext) RecordDownload(n int) {
	if s.trafficRecorder != nil {
		s.trafficRecorder.RecordDownload(n)
	}
}

// DomainFrontingAddress returns a host:port pair for a fronting domain.
func (p *Proxy) DomainFrontingAddress() string {
	return net.JoinHostPort(p.secret.Host, strconv.Itoa(p.domainFrontingPort))
}

// ServeConn serves a connection. We do not check IP blocklist and concurrency
// limit here.
func (p *Proxy) ServeConn(conn essentials.Conn) {
	p.streamWaitGroup.Add(1)
	defer p.streamWaitGroup.Done()

	ctx := newStreamContext(p.ctx, p.logger, conn)
	defer ctx.Close()

	go func() {
		<-ctx.Done()
		ctx.Close()
	}()

	// ===== 新增：用户识别 =====
    if dynamicMgr, ok := p.secret.(secret.DynamicManager); ok {
        if addr, ok := conn.LocalAddr().(*net.TCPAddr); ok {
            if userID, exists := dynamicMgr.GetUserIDByPort(addr.Port); exists {
                ctx.userID = userID
                ctx.logger = ctx.logger.BindStr("user-id", userID)
                ctx.trafficRecorder = stats.NewTrafficRecorder(userID)
                
                if speed, exists := dynamicMgr.GetUserSpeed(userID); exists && speed > 0 {
                    // 应用限速逻辑
                }
            }
        }
    }

	p.eventStream.Send(ctx, NewEventStart(ctx.streamID, ctx.ClientIP()))
	ctx.logger.Info("Stream has been started")

	defer func() {
		// 上报流量
		if ctx.userID != "" && ctx.trafficRecorder != nil {
			record := ctx.trafficRecorder.Finalize(ctx.userID)
			if reporter := events.GetReporter(); reporter != nil {
				reporter.ReportTraffic(record)
			}
		}
		
		p.eventStream.Send(ctx, NewEventFinish(ctx.streamID))
		ctx.logger.Info("Stream has been finished")
	}()

	if !p.doFakeTLSHandshake(ctx) {
		return
	}

	if err := p.doObfuscated2Handshake(ctx); err != nil {
		p.logger.InfoError("obfuscated2 handshake is failed", err)

		return
	}

	if err := p.doTelegramCall(ctx); err != nil {
		p.logger.WarningError("cannot dial to telegram", err)

		return
	}

	// 修改 relay.Relay 调用
	relay.Relay(
		ctx,
		ctx.logger.Named("relay"),
		ctx.telegramConn,
		ctx.clientConn,
		ctx, // 传递上下文用于流量记录
	)
}

// Serve starts a proxy on a given listener.
func (p *Proxy) Serve(listener net.Listener) error {
	// ... 原有代码不变 ...
}

// Shutdown 'gracefully' shutdowns all connections. Please remember that it
// does not close an underlying listener.
func (p *Proxy) Shutdown() {
	// ... 原有代码不变 ...
}

func (p *Proxy) doFakeTLSHandshake(ctx *streamContext) bool {
	// ... 原有代码不变 ...
}

func (p *Proxy) doObfuscated2Handshake(ctx *streamContext) error {
	// ... 原有代码不变 ...
}

func (p *Proxy) doTelegramCall(ctx *streamContext) error {
	// ... 原有代码不变 ...
}

func (p *Proxy) doDomainFronting(ctx *streamContext, conn *connRewind) {
	// ... 原有代码不变 ...
}

// NewProxy makes a new proxy instance.
func NewProxy(opts ProxyOpts) (*Proxy, error) {
	// ... 原有代码不变 ...
}

// 新增函数：创建流上下文
func newStreamContext(ctx context.Context, logger Logger, clientConn essentials.Conn) *streamContext {
	connIDBytes := make([]byte, ConnectionIDBytesLength)

	if _, err := rand.Read(connIDBytes); err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	streamCtx := &streamContext{
		ctx:        ctx,
		ctxCancel:  cancel,
		clientConn: clientConn,
		streamID:   base64.RawURLEncoding.EncodeToString(connIDBytes),
		trafficRecorder: stats.NewTrafficRecorder(), // 初始化流量记录器
	}
	streamCtx.logger = logger.
		BindStr("stream-id", streamCtx.streamID).
		BindStr("client-ip", streamCtx.ClientIP().String())

	return streamCtx
}

// 新增方法：获取客户端IP
func (s *streamContext) ClientIP() net.IP {
	return s.clientConn.RemoteAddr().(*net.TCPAddr).IP //nolint: forcetypeassert
}