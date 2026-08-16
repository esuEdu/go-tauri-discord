package pubsub

import (
	"context"
	"sync"
	"sync/atomic"
)

const subscriberBuffer = 64

type Memory struct {
	mu     sync.RWMutex
	topics map[string]map[*subscription]struct{}
	closed bool

	dropped atomic.Uint64
}

type subscription struct {
	ch chan []byte
}

func NewMemory() *Memory {
	return &Memory{topics: make(map[string]map[*subscription]struct{})}
}

func (m *Memory) Publish(_ context.Context, topic string, payload []byte) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil
	}
	for sub := range m.topics[topic] {
		select {
		case sub.ch <- payload:
		default:
			m.dropped.Add(1)
		}
	}
	return nil
}

func (m *Memory) Subscribe(topic string) (<-chan []byte, func()) {
	sub := &subscription{ch: make(chan []byte, subscriberBuffer)}

	m.mu.Lock()
	if m.topics[topic] == nil {
		m.topics[topic] = make(map[*subscription]struct{})
	}
	m.topics[topic][sub] = struct{}{}
	m.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			m.mu.Lock()

			alreadyClosed := m.closed
			if subs, ok := m.topics[topic]; ok {
				delete(subs, sub)
				if len(subs) == 0 {
					delete(m.topics, topic)
				}
			}
			m.mu.Unlock()
			if !alreadyClosed {
				close(sub.ch)
			}
		})
	}
	return sub.ch, cancel
}

func (m *Memory) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	for topic, subs := range m.topics {
		for sub := range subs {
			close(sub.ch)
		}
		delete(m.topics, topic)
	}
	return nil
}

func (m *Memory) Dropped() uint64 { return m.dropped.Load() }
