package collab

import (
	"context"
	"net/http"

	"github.com/coder/websocket"
	ysync "github.com/reearth/ygo/sync"
)

func (c *Collab) Handle() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var url = r.URL.RequestURI()
		var name, ok = c.doc.ExtractName(url)
		if !ok {
			c.log.Warn("verify client: name not defined", "url", url)
			http.Error(w, "name not defined", http.StatusBadRequest)
			return
		}
		var err error
		if err = c.doc.Auth(r.Context(), name, r); err != nil {
			c.log.Error("verify client", "error", err, "url", url)
			http.Error(w, "verify failed", http.StatusForbidden)
			return
		}
		if c.closed.Load() {
			http.Error(w, "service is closing", http.StatusConflict)
			return
		}
		var ws *websocket.Conn
		if ws, err = websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: c.settings.Origins}); err != nil {
			c.log.Error("accept connection", "error", err, "url", url)
			return
		}
		c.join(name, ws, r)
	}
}

func (c *Collab) join(name string, ws *websocket.Conn, r *http.Request) {
	c.mu.RLock()
	var room = c.rooms[name]
	c.mu.RUnlock()
	var first = false
	if room == nil {
		c.mutexes.lock(name)
		c.mu.RLock()
		room = c.rooms[name]
		c.mu.RUnlock()
		if room == nil {
			var err error
			var document string
			if document, err = c.doc.Get(context.Background(), name, r); err != nil {
				c.mutexes.unlock(name)
				ws.Close(websocket.StatusInternalError, "can't get document")
				return
			}
			c.mu.Lock()
			if c.closed.Load() {
				c.mu.Unlock()
				c.mutexes.unlock(name)
				ws.Close(websocket.StatusGoingAway, "")
				return
			}
			room = newRoom(c, name, document)
			first = true
			c.rooms[name] = room
			c.mu.Unlock()
		}
		c.mutexes.unlock(name)
	}

	var cn = newConn(ws, room, r)
	if !room.join(cn) {
		ws.Close(websocket.StatusGoingAway, "")
		return
	}
	defer room.wgReaders.Done()
	defer func() {
		var _ = recover()
		cn.close(websocket.StatusGoingAway, "")
	}()
	ws.SetReadLimit(c.settings.ReadLimit)
	if c.cb.OnOpenConnection != nil {
		c.cb.OnOpenConnection(name, r)
	}
	if first && c.cb.OnOpenRoom != nil {
		c.cb.OnOpenRoom(name)
	}
	go cn.writeLoop()
	go cn.pingLoop(c.settings.PingInterval, c.settings.PingTimeout)
	cn.send(syncFrame(ysync.EncodeSyncStep1(room.doc)))
	var states = room.aw.GetStates()
	if len(states) > 0 {
		var ids = make([]uint64, 0, len(states))
		for id := range states {
			ids = append(ids, id)
		}
		cn.send(awarenessFrame(room.aw.EncodeUpdate(ids)))
	}
	for {
		var typ, data, err = cn.ws.Read(context.Background())
		if err != nil {
			var status = websocket.CloseStatus(err)
			if status == -1 {
				c.log.Error("read connection", "error", err, "name", name)
				cn.close(websocket.StatusInternalError, "read error")
			} else {
				cn.close(status, "")
			}
			return
		}
		if typ == websocket.MessageText {
			cn.close(websocket.StatusUnsupportedData, "binary frames required")
			return
		}
		if err = cn.room.processFrame(data, cn); err != nil {
			c.log.Error("process frame", "error", err, "name", name)
			cn.close(websocket.StatusProtocolError, "invalid frame")
			return
		}
	}
}
