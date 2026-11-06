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

	"github.com/reearth/ygo/crdt"
	ysync "github.com/reearth/ygo/sync"
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

func TestCollabGetRoomsByUserId(t *testing.T) {
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

	var got = c.GetRoomsByUserId("alice")

	if !equalSlices(got, []string{"room1", "room2"}) {
		t.Fatalf("alice rooms = %v, want [room1 room2]", got)
	}
	if got = c.GetRoomsByUserId("bob"); !equalSlices(got, []string{"room2"}) {
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

func TestCollabGetRoomsByUserIdEmpty(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	if got := c.GetRoomsByUserId("alice"); len(got) != 0 {
		t.Fatalf("expected no rooms on fresh collab, got %v", got)
	}
	var r = newTestRoom(t, c, "room1", "hello", nil)
	c.rooms.set("room1", r)
	var cn = newTestConn(t, r, "bob")
	joinConn(t, r, cn)
	if got := c.GetRoomsByUserId("alice"); len(got) != 0 {
		t.Fatalf("expected no rooms for alice, got %v", got)
	}
	r.leave(cn)
	r.wgReaders.Done()
	waitForRoomClose(t, c)
}

func TestCollabGetRoomUsers(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var room1 = newTestRoom(t, c, "room1", "hello", nil)
	var room2 = newTestRoom(t, c, "room2", "hello", nil)
	c.rooms.set("room1", room1)
	c.rooms.set("room2", room2)

	var alice1 = newTestConn(t, room1, "alice")
	var alice2 = newTestConn(t, room1, "alice")
	var bob = newTestConn(t, room1, "bob")
	var carol = newTestConn(t, room2, "carol")
	joinConn(t, room1, alice1)
	joinConn(t, room1, alice2)
	joinConn(t, room1, bob)
	joinConn(t, room2, carol)

	var got = c.GetRoomUsers("room1")
	if !equalSlices(got, []string{"alice", "alice", "bob"}) {
		t.Fatalf("room1 users = %v, want [alice alice bob]", got)
	}
	if got = c.GetRoomUsers("room2"); !equalSlices(got, []string{"carol"}) {
		t.Fatalf("room2 users = %v, want [carol]", got)
	}
	room1.leave(alice1)
	if got = c.GetRoomUsers("room1"); !equalSlices(got, []string{"alice", "bob"}) {
		t.Fatalf("room1 users after leave = %v, want [alice bob]", got)
	}
	room1.leave(alice2)
	room1.leave(bob)
	room2.leave(carol)
	room1.wgReaders.Done()
	room1.wgReaders.Done()
	room1.wgReaders.Done()
	room2.wgReaders.Done()
	waitForRoomClose(t, c)
}

func TestCollabGetRoomUsersRoomWithoutConns(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	c.rooms.set("room1", newTestRoom(t, c, "room1", "hello", nil))
	if got := c.GetRoomUsers("room1"); len(got) != 0 {
		t.Fatalf("expected no users in a room without conns, got %v", got)
	}
}

func TestCollabGetRoomUsersRoomMissing(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	if got := c.GetRoomUsers("missing"); got != nil {
		t.Fatalf("expected nil for a missing room, got %v", got)
	}
}

func TestCollabGetRoomUsersConcurrentJoinLeave(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "room1", "hello", nil)
	c.rooms.set("room1", r)
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Go(func() {
			var cn = &conn{userId: fmt.Sprintf("user-%d", i)}
			for range 100 {
				r.conns.set(cn)
				c.GetRoomUsers("room1")
				r.conns.delete(cn)
			}
		})
	}
	for range 1000 {
		c.GetRoomUsers("room1")
	}
	wg.Wait()
	if got := len(c.GetRoomUsers("room1")); got != 0 {
		t.Fatalf("expected no users after all conns left, got %d", got)
	}
}

func TestCollabGetRoomsAll(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	c.rooms.set("room1", newTestRoom(t, c, "room1", "hello", nil))
	c.rooms.set("room2", newTestRoom(t, c, "room2", "hello", nil))
	c.rooms.set("room3", newTestRoom(t, c, "room3", "hello", nil))
	var got = c.GetRooms()
	if !equalSlices(got, []string{"room1", "room2", "room3"}) {
		t.Fatalf("rooms = %v, want all three rooms", got)
	}
}

func TestCollabGetRoomsNoRooms(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	if got := c.GetRooms(); len(got) != 0 {
		t.Fatalf("expected no rooms on fresh collab, got %v", got)
	}
}

func TestCollabHasRoom(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	c.rooms.set("room1", newTestRoom(t, c, "room1", "hello", nil))
	if !c.HasRoom("room1") {
		t.Fatal("expected HasRoom to report room1")
	}
	if c.HasRoom("missing") {
		t.Fatal("expected HasRoom to be false for a missing room")
	}
}

func TestCollabLockUnlockRoom(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	if err := c.UnlockRoom("room1"); err == nil {
		t.Fatal("expected error when unlocking a room that was never locked")
	}
	c.LockRoom("room1")
	var err error
	if err = c.UnlockRoom("room1"); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if err = c.UnlockRoom("room1"); err == nil {
		t.Fatal("expected error on a second unlock of the same room")
	}
}

func TestCollabLockRoomSerializesCallers(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	c.LockRoom("room1")
	var entered = make(chan struct{})
	go func() {
		c.LockRoom("room1")
		close(entered)
		c.UnlockRoom("room1")
	}()
	select {
	case <-entered:
		t.Fatal("second LockRoom must block while the room is locked")
	case <-time.After(100 * time.Millisecond):
	}
	if err := c.UnlockRoom("room1"); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("second LockRoom did not acquire the room after unlock")
	}
}

func TestCollabLockRoomBlocksRoomClose(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "room1", "hello", nil)
	c.rooms.set("room1", r)
	c.LockRoom("room1")
	r.close()
	if c.rooms.get("room1") == nil {
		t.Fatal("room must not be finalized while it is locked")
	}
	if err := c.UnlockRoom("room1"); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	waitForRoomClose(t, c)
	if c.rooms.get("room1") != nil {
		t.Fatal("room must be finalized after unlock")
	}
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

func TestCollabGetContentVersion(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "room1", "hello", nil)
	c.rooms.set("room1", r)
	var initial = c.GetContentVersion("room1")
	if initial == 0 {
		t.Fatal("version of a live room must not be zero")
	}
	var foreign = crdt.New(crdt.WithClientID(51))
	if err := crdt.ApplyUpdateV1(foreign, r.doc.EncodeStateAsUpdate(), nil); err != nil {
		t.Fatalf("sync foreign doc with room state: %v", err)
	}
	foreign.Transact(func(txn *crdt.Transaction) {
		txn.GetText(textName).Insert(txn, 5, " world", nil)
	}, nil)
	if _, err := c.ApplyPatch("room1", frameSync(ysync.EncodeUpdate(foreign.EncodeStateAsUpdate()))); err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if got := c.GetContentVersion("room1"); got != initial+1 {
		t.Fatalf("version = %d after one update, want %d", got, initial+1)
	}
}

func TestCollabGetContentVersionRoomMissing(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	if got := c.GetContentVersion("missing"); got != 0 {
		t.Fatalf("expected 0 for a missing room, got %d", got)
	}
}

func TestCollabGetContentVersionNoBumpOnKnownUpdate(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "room1", "hello", nil)
	c.rooms.set("room1", r)
	var initial = c.GetContentVersion("room1")
	var foreign = crdt.New(crdt.WithClientID(52))
	if err := crdt.ApplyUpdateV1(foreign, r.doc.EncodeStateAsUpdate(), nil); err != nil {
		t.Fatalf("sync foreign doc with room state: %v", err)
	}
	if _, err := c.ApplyPatch("room1", frameSync(ysync.EncodeUpdate(foreign.EncodeStateAsUpdate()))); err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if got := c.GetContentVersion("room1"); got != initial {
		t.Fatalf("version must not change on an already-known update, got %d, want %d", got, initial)
	}
}

func TestCollabGetContentSnapshot(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "room1", "hello", nil)
	c.rooms.set("room1", r)
	var version, content = c.GetContentSnapshot("room1")
	if version != c.GetContentVersion("room1") {
		t.Fatalf("snapshot version = %d, want %d", version, c.GetContentVersion("room1"))
	}
	if string(content) != "hello" {
		t.Fatalf("snapshot content = %q, want %q", content, "hello")
	}
	var foreign = crdt.New(crdt.WithClientID(53))
	if err := crdt.ApplyUpdateV1(foreign, r.doc.EncodeStateAsUpdate(), nil); err != nil {
		t.Fatalf("sync foreign doc with room state: %v", err)
	}
	foreign.Transact(func(txn *crdt.Transaction) {
		txn.GetText(textName).Insert(txn, 5, " world", nil)
	}, nil)
	if _, err := c.ApplyPatch("room1", frameSync(ysync.EncodeUpdate(foreign.EncodeStateAsUpdate()))); err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if version, content = c.GetContentSnapshot("room1"); version != c.GetContentVersion("room1") {
		t.Fatalf("snapshot version = %d, want %d", version, c.GetContentVersion("room1"))
	}
	if string(content) != "hello world" {
		t.Fatalf("snapshot content = %q, want %q", content, "hello world")
	}
}

func TestCollabGetContentSnapshotRoomMissing(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var version, content = c.GetContentSnapshot("missing")
	if version != 0 || content != nil {
		t.Fatalf("expected (0, nil) for a missing room, got (%d, %q)", version, content)
	}
}

func TestCollabGetContentSnapshotConsistentUnderConcurrentPatches(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "room1", "hello", nil)
	c.rooms.set("room1", r)
	var baseVersion, baseContent = c.GetContentSnapshot("room1")
	if string(baseContent) != "hello" {
		t.Fatalf("base content = %q, want %q", baseContent, "hello")
	}
	var baseState = r.doc.EncodeStateAsUpdate()
	var done = make(chan struct{})
	var checkers sync.WaitGroup
	for range 2 {
		checkers.Go(func() {
			for {
				select {
				case <-done:
					return
				default:
				}
				var version, content = c.GetContentSnapshot("room1")
				if version < baseVersion || len(content) != len(baseContent)+int(version-baseVersion) {
					t.Errorf("torn snapshot: version=%d content len=%d, want len=%d",
						version, len(content), len(baseContent)+int(version-baseVersion))
					return
				}
			}
		})
	}
	var patchers sync.WaitGroup
	for i := range 4 {
		patchers.Go(func() {
			var foreign = crdt.New(crdt.WithClientID(crdt.ClientID(60 + i)))
			if err := crdt.ApplyUpdateV1(foreign, baseState, nil); err != nil {
				t.Errorf("sync foreign doc with room state: %v", err)
				return
			}
			for range 25 {
				foreign.Transact(func(txn *crdt.Transaction) {
					txn.GetText(textName).Insert(txn, 0, "x", nil)
				}, nil)
				if _, err := c.ApplyPatch("room1", frameSync(ysync.EncodeUpdate(foreign.EncodeStateAsUpdate()))); err != nil {
					t.Errorf("ApplyPatch: %v", err)
					return
				}
			}
		})
	}
	patchers.Wait()
	close(done)
	checkers.Wait()
	var version, content = c.GetContentSnapshot("room1")
	if version != baseVersion+100 {
		t.Fatalf("final version = %d, want %d", version, baseVersion+100)
	}
	if len(content) != len(baseContent)+100 {
		t.Fatalf("final content len = %d, want %d", len(content), len(baseContent)+100)
	}
}

func TestCollabApplyPatch(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "room1", "hello", nil)
	c.rooms.set("room1", r)

	var foreign = crdt.New(crdt.WithClientID(21))
	if err := crdt.ApplyUpdateV1(foreign, r.doc.EncodeStateAsUpdate(), nil); err != nil {
		t.Fatalf("sync foreign doc with room state: %v", err)
	}
	foreign.Transact(func(txn *crdt.Transaction) {
		txn.GetText(textName).Insert(txn, 5, " world", nil)
	}, nil)

	var reply []byte
	var err error
	if reply, err = c.ApplyPatch("room1", frameSync(ysync.EncodeUpdate(foreign.EncodeStateAsUpdate()))); err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if len(reply) != 0 {
		t.Fatalf("update patch must not produce a reply, got %d bytes", len(reply))
	}
	if got := r.text.ToString(); got != "hello world" {
		t.Fatalf("room text = %q, want %q", got, "hello world")
	}
	if got := string(c.GetContent("room1")); got != "hello world" {
		t.Fatalf("content = %q, want %q", got, "hello world")
	}
}

func TestCollabApplyPatchBroadcastsToConns(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "room1", "hello", nil)
	c.rooms.set("room1", r)
	var cn = newTestConn(t, r, "")
	joinConn(t, r, cn)
	defer r.wgReaders.Done()
	var base = r.doc.EncodeStateAsUpdate()
	var doc1 = crdt.New(crdt.WithClientID(22))
	if err := crdt.ApplyUpdateV1(doc1, base, nil); err != nil {
		t.Fatalf("sync doc1 doc with room state: %v", err)
	}
	doc1.Transact(func(txn *crdt.Transaction) {
		txn.GetText(textName).Insert(txn, 5, " world", nil)
	}, nil)
	if _, err := c.ApplyPatch("room1", frameSync(ysync.EncodeUpdate(doc1.EncodeStateAsUpdate()))); err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	var tag, payload = readFrameFromConn(t, cn)
	if tag != messageSync {
		t.Fatalf("expected broadcast sync frame, got tag %d", tag)
	}
	var doc2 = crdt.New(crdt.WithClientID(23))
	if err := crdt.ApplyUpdateV1(doc2, base, nil); err != nil {
		t.Fatalf("sync client with room base state: %v", err)
	}

	if _, err := ysync.ApplySyncMessage(doc2, payload, nil); err != nil {
		t.Fatalf("apply broadcast: %v", err)
	}
	if got := doc2.GetText(textName).ToString(); got != "hello world" {
		t.Fatalf("client text = %q, want %q", got, "hello world")
	}
}

func TestCollabApplyPatchRejectsSyncStep1(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "room1", "hello", nil)
	c.rooms.set("room1", r)

	var foreign = crdt.New(crdt.WithClientID(24))
	if _, err := c.ApplyPatch("room1", frameSync(ysync.EncodeSyncStep1(foreign))); err == nil {
		t.Fatal("expected error for a sync step1 patch")
	}
	if got := r.text.ToString(); got != "hello" {
		t.Fatalf("room text must stay unchanged, got %q", got)
	}
}

func TestCollabApplyPatchRejectsSyncStep2(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "room1", "hello", nil)
	c.rooms.set("room1", r)

	var foreign = crdt.New(crdt.WithClientID(25))
	if err := crdt.ApplyUpdateV1(foreign, r.doc.EncodeStateAsUpdate(), nil); err != nil {
		t.Fatalf("sync foreign doc with room state: %v", err)
	}
	foreign.Transact(func(txn *crdt.Transaction) {
		txn.GetText(textName).Insert(txn, 5, " world", nil)
	}, nil)
	var step2, err = ysync.EncodeSyncStep2(foreign, ysync.EncodeSyncStep1(crdt.New(crdt.WithClientID(26))))
	if err != nil {
		t.Fatalf("encode step2: %v", err)
	}
	if _, err = c.ApplyPatch("room1", frameSync(step2)); err == nil {
		t.Fatalf("expected error for a sync step2 patch")
	}
	if got := r.text.ToString(); got != "hello" {
		t.Fatalf("room text = %q, want %q", got, "hello")
	}
}

func TestCollabApplyPatchRoomMissing(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var patch = frameSync(ysync.EncodeSyncStep1(crdt.New(crdt.WithClientID(25))))
	if _, err := c.ApplyPatch("missing", patch); err == nil {
		t.Fatal("expected error for a missing room")
	}
}

func TestCollabApplyPatchRejectsNonSyncFrame(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "room1", "hello", nil)
	c.rooms.set("room1", r)
	if _, err := c.ApplyPatch("room1", frameAwareness(nil)); err == nil {
		t.Fatal("expected error for an awareness frame")
	}
	if _, err := c.ApplyPatch("room1", nil); err == nil {
		t.Fatal("expected error for an empty patch")
	}
	if got := r.text.ToString(); got != "hello" {
		t.Fatalf("room text must stay unchanged, got %q", got)
	}
}

func TestCollabSyncDoc(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "room1", "hello", nil)
	c.rooms.set("room1", r)
	var doc = crdt.New(crdt.WithClientID(27))
	var err error
	if err = c.SyncDoc("room1", doc); err != nil {
		t.Fatalf("sync doc: %v", err)
	}
	if got := doc.GetText(textName).ToString(); got != "hello" {
		t.Fatalf("doc text = %q, want %q", got, "hello")
	}
	doc.Transact(func(txn *crdt.Transaction) {
		txn.GetText(textName).Insert(txn, 5, " world", nil)
	}, nil)
	if err = c.SyncDoc("room1", doc); err != nil {
		t.Fatalf("sync doc twice: %v", err)
	}
	if got := doc.GetText(textName).ToString(); got != "hello world" {
		t.Fatalf("doc text = %q, want %q", got, "hello world")
	}
	if got := r.text.ToString(); got != "hello world" {
		t.Fatalf("room text = %q, want %q", got, "hello world")
	}
}

func TestCollabSyncDocRoomMissing(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	if err := c.SyncDoc("missing", crdt.New(crdt.WithClientID(28))); err == nil {
		t.Fatal("expected error for a missing room")
	}
}

func TestCollabSyncDocNilDoc(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "room1", "hello", nil)
	c.rooms.set("room1", r)
	if err := c.SyncDoc("room1", nil); err == nil {
		t.Fatal("expected error for a nil doc")
	}
}

func TestCollabSyncDocNoopKeepsRoomClean(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "room1", "hello", nil)
	c.rooms.set("room1", r)
	r.dirty.Store(false)
	var doc = crdt.New(crdt.WithClientID(31))
	if err := c.SyncDoc("room1", doc); err != nil {
		t.Fatalf("sync doc: %v", err)
	}
	if r.dirty.Load() {
		t.Fatal("no-op sync must not mark the room dirty")
	}
	if got := r.text.ToString(); got != "hello" {
		t.Fatalf("room text must stay unchanged, got %q", got)
	}
}

func TestCollabSyncDocBroadcastsToConns(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "room1", "hello", nil)
	c.rooms.set("room1", r)
	var cn = newTestConn(t, r, "")
	joinConn(t, r, cn)
	defer r.wgReaders.Done()

	var base = r.doc.EncodeStateAsUpdate()
	var doc = crdt.New(crdt.WithClientID(29))
	if err := c.SyncDoc("room1", doc); err != nil {
		t.Fatalf("sync doc: %v", err)
	}
	doc.Transact(func(txn *crdt.Transaction) {
		txn.GetText(textName).Insert(txn, 5, " world", nil)
	}, nil)
	if err := c.SyncDoc("room1", doc); err != nil {
		t.Fatalf("sync doc twice: %v", err)
	}
	var tag, payload = readFrameFromConn(t, cn)
	if tag != messageSync {
		t.Fatalf("expected broadcast sync frame, got tag %d", tag)
	}
	var client = crdt.New(crdt.WithClientID(30))
	if err := crdt.ApplyUpdateV1(client, base, nil); err != nil {
		t.Fatalf("sync client with room base state: %v", err)
	}
	if _, err := ysync.ApplySyncMessage(client, payload, nil); err != nil {
		t.Fatalf("apply broadcast: %v", err)
	}
	if got := client.GetText(textName).ToString(); got != "hello world" {
		t.Fatalf("client text = %q, want %q", got, "hello world")
	}
}
