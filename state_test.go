package main

import (
	"testing"
	"time"
)

func TestStateStoreConsume(t *testing.T) {
	now := time.Now()

	newStore := func(t *testing.T) *stateStore {
		t.Helper()
		store := newStateStore()
		if !store.save("state-1", pendingLogin{browserID: "browser-a", expiresAt: now.Add(stateTTL)}) {
			t.Fatal("first save must succeed")
		}
		return store
	}

	t.Run("consumes valid state once", func(t *testing.T) {
		store := newStore(t)
		if !store.consume("state-1", "browser-a", now) {
			t.Fatal("valid state must be accepted")
		}
		if store.consume("state-1", "browser-a", now) {
			t.Fatal("replayed state must be rejected")
		}
	})

	t.Run("rejects state from another browser and keeps it", func(t *testing.T) {
		store := newStore(t)
		if store.consume("state-1", "browser-b", now) {
			t.Fatal("state bound to another browser must be rejected")
		}
		if !store.consume("state-1", "browser-a", now) {
			t.Fatal("owner browser must still be able to finish login")
		}
	})

	t.Run("rejects expired state", func(t *testing.T) {
		store := newStore(t)
		if store.consume("state-1", "browser-a", now.Add(stateTTL)) {
			t.Fatal("expired state must be rejected")
		}
	})

	t.Run("rejects unknown state", func(t *testing.T) {
		store := newStore(t)
		if store.consume("state-2", "browser-a", now) {
			t.Fatal("unknown state must be rejected")
		}
	})

	t.Run("refuses to overwrite existing state", func(t *testing.T) {
		store := newStore(t)
		if store.save("state-1", pendingLogin{browserID: "browser-b", expiresAt: now.Add(stateTTL)}) {
			t.Fatal("duplicate state must not overwrite the pending login")
		}
	})

	t.Run("cleanup removes only expired entries", func(t *testing.T) {
		store := newStore(t)
		store.save("state-old", pendingLogin{browserID: "browser-a", expiresAt: now.Add(-time.Second)})
		store.deleteExpired(now)
		if _, exists := store.entries["state-old"]; exists {
			t.Fatal("expired entry must be removed")
		}
		if _, exists := store.entries["state-1"]; !exists {
			t.Fatal("active entry must be kept")
		}
	})
}
