package collab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/reearth/ygo/crdt"
	ysync "github.com/reearth/ygo/sync"
)

func TestCollabEndToEndSync(t *testing.T) {
	var doc = &mockDoc{}
	var store = newMockStore()
	var c = newTestCollab(t, doc, store)
	var srv = httptest.NewServer(c.Handle())
	t.Cleanup(srv.Close)
	var wsURL = func(path string) string {
		return "ws" + strings.TrimPrefix(srv.URL, "http") + path
	}
	var connA = newClient(t, wsURL("/room1"))
	var docA = syncClient(t, connA, 100, "hello")
	var connB = newClient(t, wsURL("/room1"))
	var docB = syncClient(t, connB, 101, "hello")
	sendClientUpdate(t, connA, docA, 5, " world")
	readClientUpdate(t, connB, docB, "hello world")
	var rm = c.rooms.get("room1")
	if rm == nil {
		t.Fatal("room room1 not found")
	}
	if got := rm.text.ToString(); got != "hello world" {
		t.Fatalf("server text = %q, want %q", got, "hello world")
	}
}

func TestCollabEndToEndCloseOnDisconnect(t *testing.T) {
	t.Helper()
	var docService = &mockDoc{}
	var store = newMockStore()
	var c = newTestCollab(t, docService, store)
	var srv = httptest.NewServer(c.Handle())
	t.Cleanup(srv.Close)

	var conn = newClient(t, "ws"+strings.TrimPrefix(srv.URL, "http")+"/room1")
	var doc = syncClient(t, conn, 100, "hello")
	sendClientUpdate(t, conn, doc, 5, " world")
	waitForRoomText(t, c, "room1", "hello world")
	var err error
	if err = conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatalf("close: %v", err)
	}
	waitForRoomFinalized(t, c, "room1")
	var contents = docService.saveContents(t)
	if len(contents) == 0 {
		t.Fatal("expected final save on disconnect")
	}
	if got := contents[len(contents)-1]; got != "hello world" {
		t.Fatalf("final save content = %q, want %q", got, "hello world")
	}
	if store.deletes == 0 {
		t.Fatal("expected final delete state from store")
	}
}

func waitForRoomText(t *testing.T, c *Collab, name, want string) {
	t.Helper()
	var deadline = time.Now().Add(5 * time.Second)
	for {
		var room = c.rooms.get(name)
		if room != nil && room.text.ToString() == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("room %q text did not become %q in time", name, want)
		}
		time.Sleep(time.Millisecond)
	}
}

func newClient(t *testing.T, url string) *websocket.Conn {
	var conn, _, err = websocket.Dial(context.Background(), url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close(websocket.StatusNormalClosure, "") })
	return conn
}

func syncClient(t *testing.T, conn *websocket.Conn, clientID crdt.ClientID, want string) *crdt.Doc {
	t.Helper()
	var tag0, _ = readServerFrame(t, conn)
	if tag0 != messageSync {
		t.Fatalf("expected initial sync frame, got tag %d", tag0)
	}
	var doc = crdt.New(crdt.WithClientID(clientID))
	var step1 = ysync.EncodeSyncStep1(doc)
	if err := conn.Write(context.Background(), websocket.MessageBinary, frameSync(step1)); err != nil {
		t.Fatalf("write step1: %v", err)
	}
	var tag, payload = readServerFrame(t, conn)
	if tag != messageSync {
		t.Fatalf("expected sync reply, got tag %d", tag)
	}
	if _, err := ysync.ApplySyncMessage(doc, payload, nil); err != nil {
		t.Fatalf("apply sync reply: %v", err)
	}
	if got := doc.GetText(textName).ToString(); got != want {
		t.Fatalf("client text = %q, want %q", got, want)
	}
	return doc
}

func sendClientUpdate(t *testing.T, conn *websocket.Conn, doc *crdt.Doc, index int, text string) {
	t.Helper()
	var updates = make(chan []byte, 8)
	doc.OnUpdate(func(update []byte, _ any) {
		select {
		case updates <- update:
		default:
		}
	})
	doc.Transact(func(txn *crdt.Transaction) {
		txn.GetText(textName).Insert(txn, index, text, nil)
	}, nil)
	var update []byte
	select {
	case update = <-updates:
	case <-time.After(2 * time.Second):
		t.Fatal("no update captured from client doc")
	}
	if err := conn.Write(context.Background(), websocket.MessageBinary, frameSync(ysync.EncodeUpdate(update))); err != nil {
		t.Fatalf("write update: %v", err)
	}
}

func readClientUpdate(t *testing.T, conn *websocket.Conn, doc *crdt.Doc, want string) {
	t.Helper()
	var err error
	var tag, payload = readServerFrame(t, conn)
	if tag != messageSync {
		t.Fatalf("expected broadcast sync, got tag %d", tag)
	}
	if _, err = ysync.ApplySyncMessage(doc, payload, nil); err != nil {
		t.Fatalf("apply broadcast: %v", err)
	}
	if got := doc.GetText(textName).ToString(); got != want {
		t.Fatalf("client text = %q, want %q", got, want)
	}
}

func TestCollabEndToEndGetRooms(t *testing.T) {
	var doc = &mockDoc{auth: func(_ context.Context, _ string, r *http.Request) (string, error) {
		return r.Header.Get("X-User-Id"), nil
	}}
	var c = newTestCollab(t, doc, nil)
	var srv = httptest.NewServer(c.Handle())
	t.Cleanup(srv.Close)
	var wsURL = func(path string) string {
		return "ws" + strings.TrimPrefix(srv.URL, "http") + path
	}
	var connAlice1 = newClientAs(t, wsURL("/room1"), "alice")
	syncClient(t, connAlice1, 100, "hello")
	var connBob = newClientAs(t, wsURL("/room1"), "bob")
	syncClient(t, connBob, 101, "hello")
	var connAlice2 = newClientAs(t, wsURL("/room2"), "alice")
	syncClient(t, connAlice2, 102, "hello")

	var got = c.GetRooms("alice")
	if !equalSlices(got, []string{"room1", "room2"}) {
		t.Fatalf("alice rooms = %v, want [room1 room2]", got)
	}
	if got = c.GetRooms("bob"); !equalSlices(got, []string{"room1"}) {
		t.Fatalf("bob rooms = %v, want [room1]", got)
	}
	if got = c.GetRooms("carol"); len(got) != 0 {
		t.Fatalf("carol rooms = %v, want none", got)
	}
}

func newClientAs(t *testing.T, url, userId string) *websocket.Conn {
	var conn, _, err = websocket.Dial(context.Background(), url, &websocket.DialOptions{
		HTTPHeader: http.Header{"X-User-Id": []string{userId}},
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close(websocket.StatusNormalClosure, "") })
	return conn
}

func TestCollabEndToEndGetContent(t *testing.T) {
	var doc = &mockDoc{}
	var store = newMockStore()
	var c = newTestCollab(t, doc, store)
	var srv = httptest.NewServer(c.Handle())
	t.Cleanup(srv.Close)

	var connA = newClient(t, "ws"+strings.TrimPrefix(srv.URL, "http")+"/room1")
	var docA = syncClient(t, connA, 100, "hello")
	var connB = newClient(t, "ws"+strings.TrimPrefix(srv.URL, "http")+"/room1")
	var docB = syncClient(t, connB, 101, "hello")
	sendClientUpdate(t, connA, docA, 5, " world")
	readClientUpdate(t, connB, docB, "hello world")
	waitForRoomText(t, c, "room1", "hello world")
	if got := string(c.GetContent("room1")); got != "hello world" {
		t.Fatalf("content = %q, want %q", got, "hello world")
	}
	if got := c.GetContent("nope"); got != nil {
		t.Fatalf("expected nil for missing room, got %q", got)
	}
}
