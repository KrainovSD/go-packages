package collab

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/KrainovSD/go-packages/helpers"
	"github.com/reearth/ygo/awareness"
	"github.com/reearth/ygo/crdt"
	"github.com/reearth/ygo/encoding"
	ysync "github.com/reearth/ygo/sync"
)

func buildSnapshot(t *testing.T, clientID uint64, content string) []byte {
	t.Helper()
	var doc = crdt.New(crdt.WithGC(true), crdt.WithClientID(crdt.ClientID(clientID)))
	doc.Transact(func(txn *crdt.Transaction) {
		txn.GetText(textName).Insert(txn, 0, content, nil)
	}, nil)
	return doc.EncodeStateAsUpdate()
}

func TestNewRoomFromDocument(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "doc1", "initial text", nil)
	if got := r.text.ToString(); got != "initial text" {
		t.Fatalf("text = %q, want %q", got, "initial text")
	}
}

func TestNewRoomFromSnapshot(t *testing.T) {
	var snapshot = buildSnapshot(t, 42, "snapshot content")
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "doc1", "document fallback", snapshot)
	if got := r.text.ToString(); got != "snapshot content" {
		t.Fatalf("text = %q, want %q", got, "snapshot content")
	}
}

func TestNewRoomCorruptSnapshotFallback(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "doc1", "fallback text", []byte{0xDE, 0xAD, 0xBE, 0xEF})
	if got := r.text.ToString(); got != "fallback text" {
		t.Fatalf("text = %q, want fallback %q", got, "fallback text")
	}
}

func TestRoomJoinAndLeave(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "doc1", "hello", nil)
	var cn = newTestConn(t, r)
	joinConn(t, r, cn)
	r.mu.Lock()
	var n = len(r.conns)
	r.mu.Unlock()
	if n != 1 {
		t.Fatalf("expected 1 conn, got %d", n)
	}
	r.leave(cn)
	r.mu.Lock()
	n = len(r.conns)
	r.mu.Unlock()
	if n != 0 {
		t.Fatalf("expected 0 conns after leave, got %d", n)
	}
	if !r.closed.Load() {
		t.Fatal("room must close after the last conn leaves")
	}
	r.wgReaders.Done()
	waitForRoomClose(t, c)
}

func TestRoomJoinWhenClosedRejected(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "doc1", "hello", nil)
	var cn = newTestConn(t, r)
	joinConn(t, r, cn)
	r.leave(cn)
	r.wgReaders.Done()
	waitForRoomClose(t, c)
	if r.join(cn) {
		t.Fatal("join after close must be rejected")
	}
}

func TestRoomLeaveRemovesAwareness(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "doc1", "hello", nil)
	var cn = newTestConn(t, r)
	joinConn(t, r, cn)
	var aw = awareness.New(7)
	aw.SetLocalState(map[string]any{"user": "alice"})
	if err := r.processFrame(frameAwareness(aw.EncodeUpdate(nil)), cn); err != nil {
		t.Fatalf("processFrame: %v", err)
	}
	r.mu.Lock()
	var tracked = len(r.conns[cn])
	r.mu.Unlock()
	if tracked != 1 {
		t.Fatalf("expected 1 tracked awareness id, got %d", tracked)
	}
	if _, ok := r.aw.Meta(7); !ok {
		t.Fatal("expected awareness meta for client 7")
	}
	r.leave(cn)
	r.wgReaders.Done()
	waitForRoomClose(t, c)
	var states = r.aw.GetStates()
	if _, ok := states[7]; ok {
		t.Fatal("awareness state for client 7 should be removed after leave")
	}
}

func TestRoomCloseSavesAndDeletesStore(t *testing.T) {
	var doc = &mockDoc{}
	var store = newMockStore()
	var c = newTestCollab(t, doc, store)
	var r = newTestRoom(t, c, "doc1", "hello", nil)
	r.doc.Transact(func(txn *crdt.Transaction) {
		r.text.Insert(txn, 5, " world", nil)
	}, nil)
	r.close()
	waitForRoomClose(t, c)
	var contents = doc.saveContents(t)
	if len(contents) == 0 {
		t.Fatal("expected doc.Save to be called on close")
	}
	if last := contents[len(contents)-1]; !strings.Contains(last, "hello world") {
		t.Fatalf("expected final save to contain current content, got %q", last)
	}
	if store.deleteCount(t) == 0 {
		t.Fatal("expected store.Delete on successful save")
	}
}

func TestRoomCloseWritesStoreOnSaveError(t *testing.T) {
	var doc = &mockDoc{save: func(context.Context, string, string) error {
		return errors.New("save failed")
	}}
	var store = newMockStore()
	var c = newTestCollab(t, doc, store)
	var r = newTestRoom(t, c, "doc1", "hello", nil)
	r.close()
	waitForRoomClose(t, c)
	if store.writeCount(t) == 0 {
		t.Fatal("expected store.Write when doc.Save fails")
	}
	var snap = store.snapshot(t, "doc1")
	if snap == nil {
		t.Fatal("expected snapshot written to store")
	}
	var dec = crdt.New(crdt.WithGC(true), crdt.WithClientID(1))
	if err := crdt.ApplyUpdateV1(dec, snap, nil); err != nil {
		t.Fatalf("apply snapshot: %v", err)
	}
	if got := dec.GetText(textName).ToString(); got != "hello" {
		t.Fatalf("snapshot text = %q, want %q", got, "hello")
	}
}

func TestRoomCloseSaveWorkerNoDeadlock(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newRoom(c, "doc1", "hello")
	t.Cleanup(func() { r.aw.Destroy() })
	r.saveWorker.close()
	var passed = make(chan struct{})
	var proceed = make(chan struct{})
	r.saveWorker = newSerializedDebounceIntervalWorker(func() {
		if r.closed.Load() {
			return
		}
		close(passed)
		<-proceed
		c.mutexes.lock(r.name)
		c.store.Write(r.name, r.doc.EncodeStateAsUpdate())
		c.mutexes.unlock(r.name)
	}, 10*time.Millisecond, time.Hour)
	r.saveWorker.do()
	<-passed

	go func() {
		time.Sleep(100 * time.Millisecond)
		close(proceed)
	}()

	if err := helpers.LimitWork(context.Background(), 3*time.Second, func(ctx context.Context) error {
		r.close()
		c.wgRoomClose.Wait()
		return nil
	}); err != nil {
		t.Fatal("deadlock: close waits for worker blocked on the resource mutex")
	}
}

func TestRoomProcessFrameSyncStep1(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "doc1", "hello", nil)
	var cn = newTestConn(t, r)
	joinConn(t, r, cn)
	defer r.wgReaders.Done()

	var clientDoc = crdt.New(crdt.WithClientID(7))
	var step1 = ysync.EncodeSyncStep1(clientDoc)
	if err := r.processFrame(frameSync(step1), cn); err != nil {
		t.Fatalf("processFrame: %v", err)
	}
	var tag, payload = readFrameFromConn(t, cn)
	if tag != messageSync {
		t.Fatalf("expected sync reply, got tag %d", tag)
	}
	if _, err := ysync.ApplySyncMessage(clientDoc, payload, nil); err != nil {
		t.Fatalf("apply reply: %v", err)
	}
	if got := clientDoc.GetText(textName).ToString(); got != "hello" {
		t.Fatalf("client text = %q, want %q", got, "hello")
	}
}

func TestRoomProcessFrameAwareness(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "doc1", "hello", nil)
	var cn = newTestConn(t, r)
	joinConn(t, r, cn)
	defer r.wgReaders.Done()
	var aw = awareness.New(9)
	aw.SetLocalState(map[string]any{"user": "bob"})
	if err := r.processFrame(frameAwareness(aw.EncodeUpdate(nil)), cn); err != nil {
		t.Fatalf("processFrame: %v", err)
	}
	if _, ok := r.aw.Meta(9); !ok {
		t.Fatal("expected awareness meta for client 9")
	}
	r.mu.Lock()
	var tracked = len(r.conns[cn])
	r.mu.Unlock()
	if tracked != 1 {
		t.Fatalf("expected 1 tracked awareness id, got %d", tracked)
	}
}

func TestRoomProcessFrameQueryAwareness(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "doc1", "hello", nil)
	var cn = newTestConn(t, r)
	joinConn(t, r, cn)
	defer r.wgReaders.Done()
	if err := r.processFrame(frameQueryAwareness(), cn); err != nil {
		t.Fatalf("processFrame: %v", err)
	}
	var tag, _ = readFrameFromConn(t, cn)
	if tag != messageAwareness {
		t.Fatalf("expected awareness reply, got tag %d", tag)
	}
}

func TestRoomProcessFrameUnknownTag(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "doc1", "hello", nil)
	var cn = newTestConn(t, r)
	joinConn(t, r, cn)
	defer r.wgReaders.Done()
	var enc = encoding.NewEncoder()
	enc.WriteVarUint(99)
	if err := r.processFrame(enc.Bytes(), cn); err != nil {
		t.Fatalf("unknown tag must be ignored, got error %v", err)
	}
}

func TestRoomProcessFrameMalformed(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "doc1", "hello", nil)
	var cn = newTestConn(t, r)
	joinConn(t, r, cn)
	defer r.wgReaders.Done()
	if err := r.processFrame([]byte{}, cn); err == nil {
		t.Fatal("expected error for an empty frame")
	}
}

func TestRoomSaveIntervalDrivesPeriodicSave(t *testing.T) {
	var store = newMockStore()
	var c, err = New(&Options{
		Doc:      &mockDoc{},
		Store:    store,
		Log:      testLogger(t),
		Settings: Settings{SaveInterval: 50 * time.Millisecond},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	var r = newRoom(c, "doc1", "hello")
	t.Cleanup(func() { r.saveWorker.close(); r.aw.Destroy() })
	var deadline = time.Now().Add(2 * time.Second)
	for {
		if store.snapshot(t, "doc1") != nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("periodic save did not fire within SaveInterval")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestRoomOnUpdateTriggersSave(t *testing.T) {
	var store = newMockStore()
	var c = newTestCollab(t, nil, store)
	var r = newTestRoom(t, c, "doc1", "hello", nil)
	r.doc.Transact(func(txn *crdt.Transaction) {
		r.text.Insert(txn, 5, " world", nil)
	}, nil)
	var deadline = time.Now().Add(2 * time.Second)
	for {
		var snap = store.snapshot(t, "doc1")
		if snap != nil {
			var dec = crdt.New(crdt.WithClientID(1))
			if err := crdt.ApplyUpdateV1(dec, snap, nil); err == nil {
				if dec.GetText(textName).ToString() == "hello world" {
					return
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("store was not written after doc change")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRoomBroadcastSendsToConns(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "doc1", "hello", nil)
	var cn1 = newTestConn(t, r)
	var cn2 = newTestConn(t, r)
	joinConn(t, r, cn1)
	joinConn(t, r, cn2)
	defer func() { r.wgReaders.Done(); r.wgReaders.Done() }()
	r.broadcast(syncFrame([]byte("payload")))
	var tag1, payload1 = readFrameFromConn(t, cn1)
	var tag2, payload2 = readFrameFromConn(t, cn2)
	if tag1 != messageSync || string(payload1) != "payload" {
		t.Fatalf("conn1 got tag=%d payload=%q", tag1, payload1)
	}
	if tag2 != messageSync || string(payload2) != "payload" {
		t.Fatalf("conn2 got tag=%d payload=%q", tag2, payload2)
	}
}

func TestConnSendSlowConsumer(t *testing.T) {
	var c = newTestCollab(t, nil, nil)
	var r = newTestRoom(t, c, "doc1", "hello", nil)
	var cn = newTestConn(t, r)
	joinConn(t, r, cn)
	defer r.wgReaders.Done()
	for i := range frameQueueSize {
		if !cn.send([]byte{1}) {
			t.Fatalf("send %d must succeed while the buffer has space", i)
		}
	}
	if cn.send([]byte{1}) {
		t.Fatal("send must fail when the out buffer is full")
	}
	if !cn.closed.Load() {
		t.Fatal("conn must be closed as a slow consumer")
	}
}
