package proxy

import (
	"context"
	"log"
	"math"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type manager struct {
	ctx context.Context

	rate string
}

// LaunchProxy takes two net.Conns and forwards traffic between them. In terms of directionality,
// `src -> dst` is considered upload, while `dst -> src` is considered download
func (m *manager) Launch(ctx context.Context, src, dst net.Conn, l *log.Logger) error {

	tunCtx, _ := context.WithCancel(mergeContexts(m.ctx, ctx))

	tunnel := &tunnel{
		src: src,
		dst: dst,
	}

	go tunnel.run(tunCtx)
	return nil
}

// Stats track metrics about byte transfer.
type Stats struct {
	time.Time // epoch start time

	sessionsProxying int64 // Number of open Proxy connections (count - not reset)

	newBytesUp   int64 // Number of bytes transferred during epoch
	newBytesDown int64 // Number of bytes transferred during epoch

	completeBytesUp   int64 // number of bytes transferred through completed connections UP
	completeBytesDown int64 // number of bytes transferred through completed connections DOWN

	zeroByteTunnelsUp   int64 // number of closed tunnels that uploaded 0 bytes
	zeroByteTunnelsDown int64 // number of closed tunnels that downloaded 0 bytes
	completedSessions   int64 // number of completed sessions
}

// PrintAndReset implements the stats interface
func (s *Stats) PrintAndReset(logger *log.Logger) {
	s.printStats(logger)
	s.reset()
}

func (s *Stats) printStats(logger *log.Logger) {
	// prevent div by 0 if thread starvation happens
	var epochDur float64 = math.Max(float64(time.Since(s.Time).Milliseconds()), 1)

	// fmtStr := "proxy-stats: %d (%f/s) up %d (%f/s) down %d completed %d 0up %d 0down  %f avg-non-0-up, %f avg-non-0-down"
	fmtStr := "proxy-stats:%d %d %f %d %f %d %d %d %f %f"

	completedSessions := atomic.LoadInt64(&s.completedSessions)
	zbtu := atomic.LoadInt64(&s.zeroByteTunnelsUp)
	zbtd := atomic.LoadInt64(&s.zeroByteTunnelsDown)

	logger.Printf(fmtStr,
		atomic.LoadInt64(&s.sessionsProxying),
		atomic.LoadInt64(&s.newBytesUp),
		float64(atomic.LoadInt64(&s.newBytesUp))/epochDur*1000,
		atomic.LoadInt64(&s.newBytesDown),
		float64(atomic.LoadInt64(&s.newBytesDown))/epochDur*1000,
		completedSessions,
		zbtu,
		zbtd,
		float64(atomic.LoadInt64(&s.completeBytesUp))/math.Max(float64(completedSessions-zbtu), 1),
		float64(atomic.LoadInt64(&s.completeBytesDown))/math.Max(float64(completedSessions-zbtd), 1),
	)
}

// Reset implements the stats interface
func (s *Stats) Reset() {
	s.reset()
}

func (s *Stats) reset() {
	atomic.StoreInt64(&s.newBytesUp, 0)
	atomic.StoreInt64(&s.newBytesDown, 0)
	atomic.StoreInt64(&s.completeBytesUp, 0)
	atomic.StoreInt64(&s.completeBytesDown, 0)
	atomic.StoreInt64(&s.zeroByteTunnelsUp, 0)
	atomic.StoreInt64(&s.zeroByteTunnelsDown, 0)
	atomic.StoreInt64(&s.completedSessions, 0)
}

func (s *Stats) addSession() {
	atomic.AddInt64(&s.sessionsProxying, 1)
}

func (s *Stats) removeSession() {
	atomic.AddInt64(&s.sessionsProxying, -1)
}

func (s *Stats) addCompleted(nb int64, isUpload bool) {
	if isUpload {
		atomic.AddInt64(&s.completeBytesUp, nb)
		if nb == 0 {
			atomic.AddInt64(&s.zeroByteTunnelsUp, 1)
		}

		// Only add to session count on closed upload stream to prevent double count
		atomic.AddInt64(&s.completedSessions, 1)
	} else {
		atomic.AddInt64(&s.completeBytesDown, nb)
		if nb == 0 {
			atomic.AddInt64(&s.zeroByteTunnelsDown, 1)
		}
	}

}

func (s *Stats) addBytes(nb int64, isUpload bool) {
	if isUpload {
		atomic.AddInt64(&s.newBytesUp, nb)
	} else {
		atomic.AddInt64(&s.newBytesDown, nb)
	}
}

var proxyStatsInstance Stats
var proxyStatsOnce sync.Once

func initStats() {
	proxyStatsInstance = Stats{}
}

// GetStats returns our singleton for proxy stats
func GetStats() *Stats {
	return getStats()
}

// getStats returns our singleton for proxy stats
func getStats() *Stats {
	proxyStatsOnce.Do(initStats)
	return &proxyStatsInstance
}
