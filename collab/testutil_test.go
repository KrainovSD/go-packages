package collab

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/KrainovSD/go-packages/helpers"
	"github.com/coder/websocket"
	"github.com/reearth/ygo/encoding"
)

type mockDoc struct {
	mu          sync.Mutex
	extractName func(string) (string, bool)
	get         func(context.Context, string, *http.Request) (string, error)
	auth        func(context.Context, string, *http.Request) error
	save        func(context.Context, string, string) error
	saveCalls   []string
}

func (m *mockDoc) ExtractName(url string) (string, bool) {
	if m.extractName != nil {
		return m.extractName(url)
	}
	var name = strings.TrimPrefix(url, "/")
	return name, name != ""
}

func (m *mockDoc) Get(ctx context.Context, name string, r *http.Request) (string, error) {
	if m.get != nil {
		return m.get(ctx, name, r)
	}
	return "hello", nil
}

func (m *mockDoc) Auth(ctx context.Context, name string, r *http.Request) error {
	if m.auth != nil {
		return m.auth(ctx, name, r)
	}
	return nil
}

func (m *mockDoc) Save(ctx context.Context, name, content string) error {
	m.mu.Lock()
	m.saveCalls = append(m.saveCalls, content)
	m.mu.Unlock()
	if m.save != nil {
		return m.save(ctx, name, content)
	}
	return nil
}

func (m *mockDoc) saveContents(t *testing.T) []string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.saveCalls...)
}

type mockStore struct {
	mu      sync.Mutex
	data    map[string][]byte
	writes  int
	deletes int
}

func newMockStore() *mockStore {
	return &mockStore{data: make(map[string][]byte)}
}

func (s *mockStore) Read(name string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data[name]
}

func (s *mockStore) Write(name string, content []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes++
	s.data[name] = append([]byte(nil), content...)
}

func (s *mockStore) Delete(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletes++
	delete(s.data, name)
}

func (s *mockStore) snapshot(t *testing.T, name string) []byte {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.data[name]...)
}

func (s *mockStore) writeCount(t *testing.T) int {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writes
}

func (s *mockStore) deleteCount(t *testing.T) int {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deletes
}

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestCollab(t *testing.T, doc *mockDoc, store *mockStore) *Collab {
	t.Helper()
	if doc == nil {
		doc = &mockDoc{}
	}
	if store == nil {
		store = newMockStore()
	}
	var c, err = New(&Options{Doc: doc, Store: store, Log: testLogger(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func newTestRoom(t *testing.T, c *Collab, name, document string, snapshot []byte) *room {
	t.Helper()
	if snapshot != nil {
		c.store.Write(name, snapshot)
	}
	var r = newRoom(c, name, document)
	r.saveWorker.close()
	r.saveWorker = newSerializedDebounceIntervalWorker(func() {
		if r.closed.Load() {
			return
		}
		c.mutexes.lock(name)
		c.store.Write(name, r.doc.EncodeStateAsUpdate())
		c.mutexes.unlock(name)
	}, 15*time.Millisecond, 30*time.Millisecond)
	t.Cleanup(func() {
		r.saveWorker.close()
		r.aw.Destroy()
	})
	return r
}

func waitForRoomClose(t *testing.T, c *Collab) {
	t.Helper()
	if err := helpers.LimitWork(context.Background(), 5*time.Second, func(ctx context.Context) error {
		c.wgRoomClose.Wait()
		return nil
	}); err != nil {
		t.Fatal("room close goroutines did not drain in time (possible deadlock)")
	}
}

func waitTimeout(t *testing.T, d time.Duration, fn func()) {
	t.Helper()
	if err := helpers.LimitWork(context.Background(), d, func(ctx context.Context) error {
		fn()
		return nil
	}); err != nil {
		t.Fatal("operation did not complete in time (possible deadlock)")
	}
}

func waitForRoomFinalized(t *testing.T, c *Collab, name string) {
	t.Helper()
	var currentDoc *mockDoc
	var ok bool
	if currentDoc, ok = c.doc.(*mockDoc); !ok {
		t.Fatalf("exptected mockDoc struct in collab document service")
	}
	var currentStore *mockStore
	if currentStore, ok = c.store.(*mockStore); !ok {
		t.Fatalf("exptected mockStore struct in collab store")
	}
	var deadline = time.Now().Add(5 * time.Second)
	for {
		c.mu.RLock()
		var _, ok = c.rooms[name]
		c.mu.RUnlock()
		if !ok && len(currentDoc.saveContents(t)) > 0 && ((currentStore.deleteCount(t) > 0 && len(currentStore.snapshot(t, name)) == 0) || (currentStore.writeCount(t) > 0 && len(currentStore.snapshot(t, name)) > 0)) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("room %q was not finalized in time", name)
		}
		time.Sleep(time.Millisecond)
	}
}

func newServerConn(t *testing.T) (server *websocket.Conn) {
	t.Helper()
	var done = make(chan struct{})
	var serverCh = make(chan *websocket.Conn, 1)
	var srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var conn, err = websocket.Accept(w, r, nil)
		if err != nil {
			t.Error("accept:", err)
			return
		}
		serverCh <- conn
		<-done
	}))
	t.Cleanup(func() {
		close(done)
		srv.Close()
	})
	var ctx = context.Background()
	var clientConn, _, err = websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	go func() {
		for {
			if _, _, err := clientConn.Read(context.Background()); err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() { clientConn.CloseNow() })
	var serverConn *websocket.Conn
	select {
	case serverConn = <-serverCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server-side websocket")
	}
	t.Cleanup(func() { serverConn.CloseNow() })
	return serverConn
}

func newTestConn(t *testing.T, r *room) *conn {
	t.Helper()
	var serverConn = newServerConn(t)
	return newConn(serverConn, r, httptest.NewRequest("GET", "/test", nil))
}

func joinConn(t *testing.T, r *room, cn *conn) {
	t.Helper()
	if !r.join(cn) {
		t.Fatal("join rejected")
	}
}

func frameSync(payload []byte) []byte {
	var enc = encoding.NewEncoder()
	enc.WriteVarUint(messageSync)
	enc.WriteRaw(payload)
	return enc.Bytes()
}

func frameAwareness(payload []byte) []byte {
	var enc = encoding.NewEncoder()
	enc.WriteVarUint(messageAwareness)
	enc.WriteVarBytes(payload)
	return enc.Bytes()
}

func frameQueryAwareness() []byte {
	var enc = encoding.NewEncoder()
	enc.WriteVarUint(messageQueryAwareness)
	return enc.Bytes()
}

func decodeFrame(data []byte) (tag uint64, payload []byte) {
	var dec = encoding.NewDecoder(data)
	tag, _ = dec.ReadVarUint()
	payload = dec.RemainingBytes()
	return tag, payload
}

func readServerFrame(t *testing.T, ws *websocket.Conn) (tag uint64, payload []byte) {
	t.Helper()
	var ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var _, data, err = ws.Read(ctx)
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	return decodeFrame(data)
}

func readFrameFromConn(t *testing.T, cn *conn) (tag uint64, payload []byte) {
	t.Helper()
	select {
	case frame := <-cn.out:
		return decodeFrame(frame)
	case <-time.After(time.Second):
		t.Fatal("no frame in conn out channel")
		return 0, nil
	}
}
