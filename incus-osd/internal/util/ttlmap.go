package util

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type ttlVal[V any] struct {
	expiry   time.Time
	v        V
	callback func(ctx context.Context)
}

// TTLMap is a sequential-access map whose entries expire periodically.
type TTLMap[K comparable, V any] struct {
	sync.Mutex

	cancel context.CancelFunc
	m      map[K]ttlVal[V]
}

// NewTTLMap creates a map that maintains a ticker that ticks according to the given interval, and deletes entries that are past their ttl.
func NewTTLMap[K comparable, V any](ctx context.Context, interval time.Duration) *TTLMap[K, V] {
	ctx, cancel := context.WithCancel(ctx) //nolint:gosec
	t := &TTLMap[K, V]{cancel: cancel, m: map[K]ttlVal[V]{}}
	ticker := time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-ticker.C:
				t.Lock()

				now := time.Now().UTC()
				for k, v := range t.m {
					if now.After(v.expiry) {
						slog.Debug("Cached record expired", slog.Time("added", v.expiry))
						delete(t.m, k)

						if v.callback != nil {
							go v.callback(ctx)
						}
					}
				}
				t.Unlock()
			case <-ctx.Done():
				ticker.Stop()

				return
			}
		}
	}()

	return t
}

// Stop the expiration of entries (map still functions).
func (t *TTLMap[K, V]) Stop() {
	t.cancel()
}

// Set a value for a particular key, with a given ttl. The callback will be executed after the entry is removed, if given.
func (t *TTLMap[K, V]) Set(k K, v V, ttl time.Duration, callback func(context.Context)) {
	t.Lock()
	defer t.Unlock()

	t.m[k] = ttlVal[V]{
		expiry:   time.Now().UTC().Add(ttl),
		v:        v,
		callback: callback,
	}
}

// Get an entry by its key.
func (t *TTLMap[K, V]) Get(k K) (V, bool) {
	t.Lock()
	defer t.Unlock()

	v, ok := t.m[k]
	if !ok || time.Now().UTC().After(v.expiry) {
		var v V

		return v, false
	}

	return v.v, true
}

// Delete an entry by its key.
func (t *TTLMap[K, V]) Delete(k K) {
	t.Lock()
	defer t.Unlock()

	delete(t.m, k)
}
