package application

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type memorySportsCache struct {
	mu      sync.Mutex
	payload []byte
	at      time.Time
}

func (c *memorySportsCache) Get(context.Context, string) ([]byte, time.Time, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.payload == nil {
		return nil, time.Time{}, false, nil
	}
	return append([]byte(nil), c.payload...), c.at, true, nil
}

func (c *memorySportsCache) Set(_ context.Context, _ string, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.payload = append([]byte(nil), payload...)
	c.at = time.Now().UTC()
	return nil
}

func (c *memorySportsCache) Delete(context.Context, string) error { return nil }

func TestSportsCacheMissIsSingleFlight(t *testing.T) {
	cache := &memorySportsCache{}
	service := &SportsService{Cache: cache, refreshing: map[string]bool{}, cacheOwner: "test"}
	var calls atomic.Int32
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			value, _, err := getOrFetch(service, context.Background(), "same-key", time.Hour, "test", nil,
				func(context.Context) (map[string]int, error) {
					calls.Add(1)
					time.Sleep(25 * time.Millisecond)
					return map[string]int{"value": 42}, nil
				})
			if err != nil || value["value"] != 42 {
				t.Errorf("value=%v err=%v", value, err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("want one upstream fetch, got %d", calls.Load())
	}
}
