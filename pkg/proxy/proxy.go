package proxy

import (
	"encoding/json"
	"log"
	"net"
	"sync/atomic"
)

type tunnel struct {
	*tunnelStats
	src, dst net.Conn
	logger   *log.Logger
	tag      string
}

type tunnelStats struct {
	proxyStats *Stats

	Duration  int64
	BytesUp   int64
	BytesDown int64

	CovertDialErr string
	CovertConnErr string
	ClientConnErr string

	PhantomAddr    string
	PhantomDstPort uint

	TunnelCount   uint
	V6            bool
	ASN           uint   `json:",omitempty"`
	CC            string `json:",omitempty"`
	Transport     string `json:",omitempty"`
	Registrar     string `json:",omitempty"`
	LibVer        uint
	Gen           uint
	TransportOpts []string `json:",omitempty"`
	RegOpts       []string `json:",omitempty"`
	Tags          []string `json:",omitempty"`
}

func (ts *tunnelStats) Print(logger *log.Logger) {
	tunStatsStr, _ := json.Marshal(ts)
	logger.Printf("proxy closed %s", tunStatsStr)
}

func (ts *tunnelStats) completed(isUpload bool) {
	if isUpload {
		ts.proxyStats.addCompleted(ts.BytesUp, isUpload)
	} else {
		ts.proxyStats.addCompleted(ts.BytesDown, isUpload)
	}
}

func (ts *tunnelStats) duration(duration int64, isUpload bool) {
	// only set duration once, so that the first to close gives us the (real) lower bound on tunnel
	// duration.
	if atomic.LoadInt64(&ts.Duration) == 0 {
		atomic.StoreInt64(&ts.Duration, duration)
	}
}

func (ts *tunnelStats) addBytes(n int64, isUpload bool) {
	if isUpload {
		atomic.AddInt64(&ts.BytesUp, n)
	} else {
		atomic.AddInt64(&ts.BytesDown, n)
	}
	ts.proxyStats.addBytes(int64(n), isUpload)
}
