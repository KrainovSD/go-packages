package collab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestCollabClose(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "room1", "hello", nil)
	c.rooms.set("room1", r)
	var ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.Close(ctx)
	c.mu.RLock()
	if !c.closed {
		c.mu.RUnlock()
		t.Fatal("collab must be marked closed")
	}
	c.mu.RUnlock()
	var room = c.rooms.get("room1")
	if room != nil {
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
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	var resp, err = http.Get(srv.URL + "/room1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestRoomsConcurrentSetAllPresent(t *testing.T) {
	var rs = newRooms()
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			for j := range 200 {
				rs.set(fmt.Sprintf("room-%d-%d", i, j), &room{name: "x"})
			}
		})
	}
	wg.Wait()
	var m = rs.val()
	if len(m) != 8*200 {
		t.Fatalf("expected %d rooms, got %d (lost updates)", 8*200, len(m))
	}
}

func TestRoomsConcurrentSetDeleteOwn(t *testing.T) {
	var rs = newRooms()
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			for j := range 100 {
				var name = fmt.Sprintf("room-%d-%d", i, j)
				var rm = &room{name: name}
				rs.set(name, rm)
				rs.delete(name, rm)
			}
		})
	}
	wg.Wait()
	if got := len(rs.val()); got != 0 {
		t.Fatalf("expected all rooms deleted, got %d", got)
	}
}

func TestRoomsDeleteWrongInstanceIsNoop(t *testing.T) {
	var rs = newRooms()
	var a = &room{name: "room"}
	rs.set("room", a)
	rs.delete("room", &room{name: "room"})
	if got := len(rs.val()); got != 1 {
		t.Fatalf("expected room kept after stale delete, got %d", got)
	}
	rs.delete("room", a)
	if got := len(rs.val()); got != 0 {
		t.Fatalf("expected room deleted, got %d", got)
	}
}
