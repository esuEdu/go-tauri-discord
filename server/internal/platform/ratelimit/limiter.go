package ratelimit

import (
	"math"
	"sync"
	"time"
)

type Policy struct {
	Every time.Duration
	Burst int
}

func PerMinute(n int, burst int) Policy {
	return per(time.Minute, n, burst)
}

func PerHour(n int, burst int) Policy {
	return per(time.Hour, n, burst)
}

func per(window time.Duration, n int, burst int) Policy {
	if n < 1 {
		n = 1
	}
	return Policy{Every: window / time.Duration(n), Burst: burst}
}

type bucket struct {
	tokens float64
	last   time.Time
}

type Limiter struct {
	policy Policy
	now    func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket

	stop chan struct{}
	once sync.Once
}

const sweepInterval = 5 * time.Minute

func New(policy Policy) *Limiter {
	if policy.Burst < 1 {
		policy.Burst = 1
	}
	l := &Limiter{
		policy:  policy,
		now:     time.Now,
		buckets: make(map[string]*bucket),
		stop:    make(chan struct{}),
	}
	go l.sweep()
	return l
}

func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(l.policy.Burst), last: now}
		l.buckets[key] = b
	}

	refill := now.Sub(b.last).Seconds() / l.policy.Every.Seconds()
	b.tokens = math.Min(float64(l.policy.Burst), b.tokens+refill)
	b.last = now

	if b.tokens < 1 {
		deficit := 1 - b.tokens
		return false, time.Duration(deficit * float64(l.policy.Every))
	}
	b.tokens--
	return true, 0
}

func (l *Limiter) Stop() {
	l.once.Do(func() { close(l.stop) })
}

func (l *Limiter) sweep() {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-l.stop:
			return
		case <-ticker.C:
			l.evictFull()
		}
	}
}

func (l *Limiter) evictFull() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	full := float64(l.policy.Burst)
	for key, b := range l.buckets {
		refill := now.Sub(b.last).Seconds() / l.policy.Every.Seconds()
		if b.tokens+refill >= full {
			delete(l.buckets, key)
		}
	}
}

func (l *Limiter) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
