package collab

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCollabClose(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "room1", "hello", nil)
	c.mu.Lock()
	c.rooms["room1"] = r
	c.mu.Unlock()
	var ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.Close(ctx)
	if !c.closed.Load() {
		t.Fatal("collab must be marked closed")
	}
	c.mu.RLock()
	var _, ok = c.rooms["room1"]
	c.mu.RUnlock()
	if ok {
		t.Fatal("room must be removed after Close")
	}
	waitForRoomClose(t, c)
}

func TestCollabCloseIsIdempotent(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.Close(ctx)
	c.Close(ctx)
}

func TestHandleNameNotDefined(t *testing.T) {
	var doc = &mockDoc{extractName: func(string) (string, bool) { return "", false }}
	var c = newTestCollab(t, doc, nil)
	var srv = httptest.NewServer(c.Handle())
	t.Cleanup(srv.Close)

	var resp, err = http.Get(srv.URL + "/room1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandleAuthFailure(t *testing.T) {
	var doc = &mockDoc{auth: func(context.Context, string, *http.Request) error {
		return errors.New("unauthorized")
	}}
	var c = newTestCollab(t, doc, nil)
	var srv = httptest.NewServer(c.Handle())
	t.Cleanup(srv.Close)

	var resp, err = http.Get(srv.URL + "/room1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestHandleClosedRejects(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var srv = httptest.NewServer(c.Handle())
	t.Cleanup(srv.Close)

	c.closed.Store(true)
	var resp, err = http.Get(srv.URL + "/room1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}
