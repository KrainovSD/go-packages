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
	var doc = &mockDoc{auth: func(context.Context, string, *http.Request) (string, error) {
		return "", errors.New("unauthorized")
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

func TestCollabGetRooms(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var room1 = newTestRoom(t, c, "room1", "hello", nil)
	var room2 = newTestRoom(t, c, "room2", "hello", nil)
	c.rooms.set("room1", room1)
	c.rooms.set("room2", room2)

	var alice1 = newTestConn(t, room1, "alice")
	var alice2 = newTestConn(t, room1, "alice")
	var alice3 = newTestConn(t, room2, "alice")
	var bob = newTestConn(t, room2, "bob")
	joinConn(t, room1, alice1)
	joinConn(t, room1, alice2)
	joinConn(t, room2, alice3)
	joinConn(t, room2, bob)

	var got = c.GetRooms("alice")

	if !equalSlices(got, []string{"room1", "room2"}) {
		t.Fatalf("alice rooms = %v, want [room1 room2]", got)
	}
	if got = c.GetRooms("bob"); !equalSlices(got, []string{"room2"}) {
		t.Fatalf("bob rooms = %v, want [room2]", got)
	}

	room1.leave(alice1)
	room1.leave(alice2)
	room2.leave(alice3)
	room2.leave(bob)
	room1.wgReaders.Done()
	room1.wgReaders.Done()
	room2.wgReaders.Done()
	room2.wgReaders.Done()
	waitForRoomClose(t, c)
}

func TestCollabGetRoomsEmpty(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	if got := c.GetRooms("alice"); len(got) != 0 {
		t.Fatalf("expected no rooms on fresh collab, got %v", got)
	}
	var r = newTestRoom(t, c, "room1", "hello", nil)
	c.rooms.set("room1", r)
	var cn = newTestConn(t, r, "bob")
	joinConn(t, r, cn)
	if got := c.GetRooms("alice"); len(got) != 0 {
		t.Fatalf("expected no rooms for alice, got %v", got)
	}
	r.leave(cn)
	r.wgReaders.Done()
	waitForRoomClose(t, c)
}

func TestCollabGetContent(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "room1", "hello", nil)
	c.rooms.set("room1", r)
	if got := string(c.GetContent("room1")); got != "hello" {
		t.Fatalf("content = %q, want %q", got, "hello")
	}
	if got := c.GetContent("missing"); got != nil {
		t.Fatalf("expected nil for missing room, got %q", got)
	}
}
