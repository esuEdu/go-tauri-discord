package pubsub

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestPublishReachesEverySubscriber(t *testing.T) {
	b := NewMemory()
	defer b.Close()

	a, cancelA := b.Subscribe("guild:1")
	defer cancelA()
	c, cancelC := b.Subscribe("guild:1")
	defer cancelC()
	other, cancelOther := b.Subscribe("guild:2")
	defer cancelOther()

	if err := b.Publish(context.Background(), "guild:1", []byte("hello")); err != nil {
		t.Fatal(err)
	}

	for i, ch := range []<-chan []byte{a, c} {
		select {
		case got := <-ch:
			if string(got) != "hello" {
				t.Errorf("subscriber %d got %q", i, got)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d received nothing", i)
		}
	}

	select {
	case got := <-other:
		t.Errorf("subscriber on another topic received %q", got)
	default:
	}
}

func TestPublishDoesNotBlockOnAFullSubscriber(t *testing.T) {
	b := NewMemory()
	defer b.Close()

	_, cancel := b.Subscribe("guild:1")
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range subscriberBuffer + 10 {
			_ = b.Publish(context.Background(), "guild:1", []byte("x"))
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked on a subscriber that stopped draining")
	}
	if b.Dropped() == 0 {
		t.Error("overflowing payloads should have been counted as dropped")
	}
}

func TestCancelUnsubscribesAndIsIdempotent(t *testing.T) {
	b := NewMemory()
	defer b.Close()

	ch, cancel := b.Subscribe("guild:1")
	cancel()
	cancel()

	if _, open := <-ch; open {
		t.Error("cancel should close the subscriber channel")
	}
	if err := b.Publish(context.Background(), "guild:1", []byte("x")); err != nil {
		t.Fatal(err)
	}
}

func TestCloseThenCancelDoesNotPanic(t *testing.T) {
	b := NewMemory()
	_, cancel := b.Subscribe("guild:1")

	b.Close()
	cancel()
}

func TestConcurrentSubscribeAndPublish(t *testing.T) {
	b := NewMemory()
	defer b.Close()

	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ch, cancel := b.Subscribe("guild:1")
			defer cancel()
			go func() {
				for range ch {
				}
			}()
			for range 50 {
				_ = b.Publish(context.Background(), "guild:1", []byte("x"))
			}
		}(i)
	}
	wg.Wait()
}
