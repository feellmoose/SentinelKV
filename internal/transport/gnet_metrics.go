package transport

import (
	"sync/atomic"
	"time"
)

// GnetMetrics provides real-time metrics for gnet transport
// ZERO-OVERHEAD: All metrics use atomic operations
type GnetMetrics struct {
	// Connection metrics
	ActiveConnections   atomic.Int64
	TotalConnections    atomic.Int64
	FailedConnections   atomic.Int64
	
	// Message metrics
	MessagesSent        atomic.Int64
	MessagesReceived    atomic.Int64
	BytesSent           atomic.Int64
	BytesReceived       atomic.Int64
	
	// Error metrics
	WriteErrors         atomic.Int64
	ReadErrors          atomic.Int64
	HandlerErrors       atomic.Int64
	
	// Performance metrics
	LastLatency         atomic.Int64  // nanoseconds
	AvgLatency          atomic.Int64  // nanoseconds
	TotalLatencySum     atomic.Int64
	LatencySamples      atomic.Int64
	
	// Timing
	StartTime           time.Time
}

// NewGnetMetrics creates a new metrics instance
func NewGnetMetrics() *GnetMetrics {
	return &GnetMetrics{
		StartTime: time.Now(),
	}
}

// RecordWrite records a write operation
func (m *GnetMetrics) RecordWrite(bytes int64, latency time.Duration) {
	m.MessagesSent.Add(1)
	m.BytesSent.Add(bytes)
	
	// Update latency metrics
	latencyNs := latency.Nanoseconds()
	m.LastLatency.Store(latencyNs)
	m.TotalLatencySum.Add(latencyNs)
	samples := m.LatencySamples.Add(1)
	
	// Update average (atomic)
	avgLatency := m.TotalLatencySum.Load() / samples
	m.AvgLatency.Store(avgLatency)
}

// RecordRead records a read operation
func (m *GnetMetrics) RecordRead(bytes int64) {
	m.MessagesReceived.Add(1)
	m.BytesReceived.Add(bytes)
}

// RecordError records an error
func (m *GnetMetrics) RecordError(errType string) {
	switch errType {
	case "write":
		m.WriteErrors.Add(1)
	case "read":
		m.ReadErrors.Add(1)
	case "handler":
		m.HandlerErrors.Add(1)
	}
}

// RecordConnection records connection events
func (m *GnetMetrics) RecordConnection(delta int64) {
	m.ActiveConnections.Add(delta)
	if delta > 0 {
		m.TotalConnections.Add(delta)
	}
}

// RecordConnectionFailed records a failed connection
func (m *GnetMetrics) RecordConnectionFailed() {
	m.FailedConnections.Add(1)
}

// Snapshot returns a snapshot of current metrics
type MetricsSnapshot struct {
	ActiveConnections   int64
	TotalConnections    int64
	FailedConnections   int64
	MessagesSent        int64
	MessagesReceived    int64
	BytesSent           int64
	BytesReceived       int64
	WriteErrors         int64
	ReadErrors          int64
	HandlerErrors       int64
	LastLatencyUs       int64
	AvgLatencyUs        int64
	Uptime              time.Duration
	MessagesPerSec      float64
	MBPerSec            float64
}

// Snapshot returns current metrics snapshot
func (m *GnetMetrics) Snapshot() MetricsSnapshot {
	uptime := time.Since(m.StartTime)
	seconds := uptime.Seconds()
	
	sent := m.MessagesSent.Load()
	bytesSent := m.BytesSent.Load()
	
	return MetricsSnapshot{
		ActiveConnections:  m.ActiveConnections.Load(),
		TotalConnections:   m.TotalConnections.Load(),
		FailedConnections:  m.FailedConnections.Load(),
		MessagesSent:       sent,
		MessagesReceived:   m.MessagesReceived.Load(),
		BytesSent:          bytesSent,
		BytesReceived:      m.BytesReceived.Load(),
		WriteErrors:        m.WriteErrors.Load(),
		ReadErrors:         m.ReadErrors.Load(),
		HandlerErrors:      m.HandlerErrors.Load(),
		LastLatencyUs:      m.LastLatency.Load() / 1000,
		AvgLatencyUs:       m.AvgLatency.Load() / 1000,
		Uptime:             uptime,
		MessagesPerSec:     float64(sent) / seconds,
		MBPerSec:           float64(bytesSent) / seconds / 1024 / 1024,
	}
}

