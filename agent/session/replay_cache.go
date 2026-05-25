package session

import (
	"encoding/hex"
	"sync"
	"time"
)

const (
	replayCacheTTL             = 120 * time.Second
	replayCacheCleanupInterval = 30 * time.Second
)

type replayCache struct {
	mu   sync.Mutex
	seen map[string]time.Time // key: hex(clientRandom), value: expiry time
	stop chan struct{}
}

func newReplayCache() *replayCache {
	c := &replayCache{
		seen: make(map[string]time.Time),
		stop: make(chan struct{}),
	}
	go c.cleanLoop()
	return c
}

// CheckAndAdd returns true (should reject) if clientRandom was already seen;
// otherwise records it and returns false.
func (c *replayCache) CheckAndAdd(clientRandom []byte, now time.Time) bool {
	key := hex.EncodeToString(clientRandom)
	expiry := now.Add(replayCacheTTL)

	c.mu.Lock()
	defer c.mu.Unlock()

	if exp, exists := c.seen[key]; exists && now.Before(exp) {
		return true
	}

	c.seen[key] = expiry
	return false
}

func (c *replayCache) cleanLoop() {
	ticker := time.NewTicker(replayCacheCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanExpired(time.Now())
		case <-c.stop:
			return
		}
	}
}

func (c *replayCache) cleanExpired(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for k, exp := range c.seen {
		if now.After(exp) {
			delete(c.seen, k)
		}
	}
}

var globalReplayCache = newReplayCache()
