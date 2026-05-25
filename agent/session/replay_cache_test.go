package session

import (
	"testing"
	"time"
)

func TestReplayCacheAllowsExpiredEntryWithoutInlineSweep(t *testing.T) {
	cache := &replayCache{
		seen: make(map[string]time.Time),
		stop: make(chan struct{}),
	}

	now := time.Unix(1_700_000_000, 0)
	clientRandom := []byte("0123456789abcdef0123456789abcdef")

	if cache.CheckAndAdd(clientRandom, now) {
		t.Fatal("first insert should not be treated as replay")
	}
	if !cache.CheckAndAdd(clientRandom, now.Add(time.Second)) {
		t.Fatal("entry should be treated as replay within ttl")
	}
	if cache.CheckAndAdd(clientRandom, now.Add(replayCacheTTL+time.Second)) {
		t.Fatal("expired entry should be accepted again")
	}
}

func TestReplayCacheCleanExpiredRemovesOldEntries(t *testing.T) {
	cache := &replayCache{
		seen: map[string]time.Time{
			"expired": time.Unix(100, 0),
			"active":  time.Unix(200, 0),
		},
		stop: make(chan struct{}),
	}

	cache.cleanExpired(time.Unix(150, 0))

	if _, ok := cache.seen["expired"]; ok {
		t.Fatal("expected expired entry to be removed")
	}
	if _, ok := cache.seen["active"]; !ok {
		t.Fatal("expected active entry to remain")
	}
}
