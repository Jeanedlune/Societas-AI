// Package eventbus provides bounded, Control Plane mediated event delivery.
package eventbus

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Jeanedlune/Societas-AI/internal/eventstore"
	"github.com/Jeanedlune/Societas-AI/internal/outbox"
)

var (
	ErrClosed       = errors.New("subscription is closed")
	ErrNotPersisted = errors.New("event is not present in the Event Store")
)

type BackpressureError struct{}

func (BackpressureError) Error() string        { return "subscriber queue is full" }
func (BackpressureError) DeliveryCode() string { return "BACKPRESSURE" }

var ErrBackpressure error = BackpressureError{}

type SubscriptionSpec struct {
	ID          string
	WorkspaceID string
	RunID       string
	Addresses   []string
	Buffer      int
}

type Subscription struct {
	bus    *Bus
	spec   SubscriptionSpec
	ch     chan eventstore.StoredEvent
	closed bool
	last   map[string]uint64
}

func (s *Subscription) Events() <-chan eventstore.StoredEvent { return s.ch }
func (s *Subscription) Close() {
	if s != nil && s.bus != nil {
		s.bus.unsubscribe(s)
	}
}

type Bus struct {
	events *eventstore.Store
	outbox *outbox.Store
	mu     sync.Mutex
	subs   map[string]*Subscription
}

func New(events *eventstore.Store, outboxStore *outbox.Store) (*Bus, error) {
	if events == nil || outboxStore == nil {
		return nil, fmt.Errorf("event bus requires event store and outbox")
	}
	return &Bus{events: events, outbox: outboxStore, subs: make(map[string]*Subscription)}, nil
}

// Publish validates through Event Store and returns only after event and outbox commit.
func (b *Bus) Publish(ctx context.Context, raw []byte) (eventstore.AppendResult, error) {
	return b.events.Append(ctx, raw)
}

func (b *Bus) Subscribe(spec SubscriptionSpec) (*Subscription, error) {
	if strings.TrimSpace(spec.ID) == "" || spec.Buffer < 1 {
		return nil, fmt.Errorf("subscription ID and positive bounded buffer are required")
	}
	spec.Addresses = append([]string(nil), spec.Addresses...)
	s := &Subscription{bus: b, spec: spec, ch: make(chan eventstore.StoredEvent, spec.Buffer), last: make(map[string]uint64)}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.subs[spec.ID]; exists {
		return nil, fmt.Errorf("subscription ID already exists")
	}
	b.subs[spec.ID] = s
	return s, nil
}

func (b *Bus) unsubscribe(s *Subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	if b.subs[s.spec.ID] == s {
		delete(b.subs, s.spec.ID)
	}
	close(s.ch)
}

// Deliver is the outbox adapter. Only persisted StoredEvents reach this method.
func (b *Bus) Deliver(ctx context.Context, event eventstore.StoredEvent) error {
	persisted, err := b.events.GetByID(ctx, string(event.Event.Header.ID))
	if err != nil || persisted.Sequence != event.Sequence {
		return ErrNotPersisted
	}
	event = persisted
	b.mu.Lock()
	defer b.mu.Unlock()
	var failed bool
	for _, s := range b.subs {
		if s.closed || !matches(s, event) {
			continue
		}
		stream := string(event.Event.Header.WorkspaceID) + "/" + string(event.Event.Header.RunID)
		if event.Sequence <= s.last[stream] {
			continue
		}
		select {
		case s.ch <- event:
			s.last[stream] = event.Sequence
		default:
			failed = true
		}
	}
	if failed {
		return ErrBackpressure
	}
	return nil
}

func matches(s *Subscription, event eventstore.StoredEvent) bool {
	h := event.Event.Header
	if s.spec.WorkspaceID != "" && s.spec.WorkspaceID != string(h.WorkspaceID) {
		return false
	}
	if s.spec.RunID != "" && s.spec.RunID != string(h.RunID) {
		return false
	}
	if len(s.spec.Addresses) == 0 {
		return true
	}
	for _, address := range s.spec.Addresses {
		if (address == "topic:all" && strings.HasPrefix(h.To, "topic:")) || address == h.To {
			return true
		}
	}
	return false
}

// ReplayRun enqueues persisted history in sequence order after the supplied cursor.
func (b *Bus) ReplayRun(ctx context.Context, subscriptionID, workspaceID, runID string, after uint64, limit int) error {
	events, err := b.events.ReadRun(ctx, workspaceID, runID, after, limit)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.subs[subscriptionID]
	if s == nil || s.closed {
		return ErrClosed
	}
	for _, event := range events {
		if !matches(s, event) {
			continue
		}
		stream := workspaceID + "/" + runID
		if event.Sequence <= s.last[stream] {
			continue
		}
		select {
		case s.ch <- event:
			s.last[stream] = event.Sequence
		default:
			return ErrBackpressure
		}
	}
	return nil
}

var _ outbox.Deliverer = (*Bus)(nil)
