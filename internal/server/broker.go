package server

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/alert"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

// SSEEvent represents a single event payload sent to SSE clients
type SSEEvent struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
	Time  string `json:"timestamp"`
}

// Broker manages active SSE client channels and event dispatching
type Broker struct {
	mu                sync.RWMutex
	clients           map[chan SSEEvent]struct{}
	stopCh            chan struct{}
	stopped           bool
	lastProfilesState string
	lastAlertsState   string
}

// NewBroker initializes an SSE event broker
func NewBroker() *Broker {
	return &Broker{
		clients: make(map[chan SSEEvent]struct{}),
		stopCh:  make(chan struct{}),
	}
}

// Start begins the background monitoring and heartbeat loop
func (b *Broker) Start(pollInterval time.Duration) {
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	go b.runLoop(pollInterval)
}

// Stop gracefully shuts down the broker and closes all client channels
func (b *Broker) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stopped {
		return
	}
	b.stopped = true
	close(b.stopCh)
	for ch := range b.clients {
		close(ch)
		delete(b.clients, ch)
	}
}

// Register registers a new client channel to receive events
func (b *Broker) Register(ch chan SSEEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.stopped {
		b.clients[ch] = struct{}{}
	}
}

// Unregister removes a client channel and closes it safely
func (b *Broker) Unregister(ch chan SSEEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.clients[ch]; ok {
		delete(b.clients, ch)
		close(ch)
	}
}

// ClientCount returns the number of active connected clients
func (b *Broker) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

// Broadcast sends an event to all connected clients non-blockingly
func (b *Broker) Broadcast(ev SSEEvent) {
	if ev.Time == "" {
		ev.Time = time.Now().UTC().Format(time.RFC3339)
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- ev:
		default:
			// client buffer full, drop event to prevent head-of-line blocking
		}
	}
}

// CheckProfilesChange checks for changes in profile list or running state and broadcasts if changed
func (b *Broker) CheckProfilesChange() {
	profiles, err := profile.GetProfiles()
	if err != nil {
		return
	}
	if profiles == nil {
		profiles = []profile.ProfileInfo{}
	}

	data, err := json.Marshal(profiles)
	if err != nil {
		return
	}
	raw := string(data)

	b.mu.Lock()
	changed := (raw != b.lastProfilesState)
	b.lastProfilesState = raw
	b.mu.Unlock()

	if changed {
		b.Broadcast(SSEEvent{
			Event: "profiles",
			Data:  profiles,
			Time:  time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// CheckAlerts evaluates quota history and reaps stale headless processes.
// Connected clients receive an alerts event only when that set changes.
func (b *Broker) CheckAlerts() {
	report, err := alert.Evaluate("")
	if err != nil || report == nil {
		return
	}
	if report.Alerts == nil {
		report.Alerts = []alert.Alert{}
	}
	data, err := json.Marshal(report.Alerts)
	if err != nil {
		return
	}
	raw := string(data)

	b.mu.Lock()
	if b.lastAlertsState == "" && raw == "[]" {
		b.lastAlertsState = raw
		b.mu.Unlock()
		return
	}
	changed := raw != b.lastAlertsState
	b.lastAlertsState = raw
	b.mu.Unlock()
	if !changed {
		return
	}
	b.Broadcast(SSEEvent{
		Event: "alerts",
		Data:  report.Alerts,
		Time:  time.Now().UTC().Format(time.RFC3339),
	})
}

func (b *Broker) runLoop(pollInterval time.Duration) {
	ticker := time.NewTicker(pollInterval)
	pingTicker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	defer pingTicker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-pingTicker.C:
			if b.ClientCount() > 0 {
				b.Broadcast(SSEEvent{
					Event: "ping",
					Data:  map[string]any{"timestamp": time.Now().UTC().Format(time.RFC3339)},
					Time:  time.Now().UTC().Format(time.RFC3339),
				})
			}
		case <-ticker.C:
			if b.ClientCount() == 0 {
				continue
			}
			b.CheckProfilesChange()
			b.CheckAlerts()
		}
	}
}
