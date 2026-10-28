package collab

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/reearth/ygo/awareness"
	"github.com/reearth/ygo/crdt"
	"github.com/reearth/ygo/encoding"
	ysync "github.com/reearth/ygo/sync"
)

const awarenessExpiryTimeout = 30 * time.Second

type room struct {
	c          *Collab
	name       string
	saveWorker *serializedDebounceIntervalWorker
	doc        *crdt.Doc
	text       *crdt.YText
	aw         *awareness.Awareness
	mu         sync.Mutex
	conns      map[*conn]map[uint64]struct{}
	closed     atomic.Bool
	wgReaders  sync.WaitGroup
}

func newRoom(c *Collab, name string, document string) *room {
	var doc = crdt.New(crdt.WithGC(true), crdt.WithClientID(crdt.NewClientID()))
	var snapshot = c.store.Read(name)
	if snapshot != nil {
		if err := crdt.ApplyUpdateV1(doc, snapshot, nil); err != nil {
			c.log.Error("apply snapshot", "error", err, "name", name)
			doc = crdt.New(crdt.WithGC(true), crdt.WithClientID(crdt.NewClientID()))
			doc.Transact(func(txn *crdt.Transaction) {
				txn.GetText(textName).Insert(txn, 0, document, nil)
			}, nil)
		}
	} else {
		doc.Transact(func(txn *crdt.Transaction) {
			txn.GetText(textName).Insert(txn, 0, document, nil)
		}, nil)
	}
	var r = &room{
		c:     c,
		name:  name,
		doc:   doc,
		text:  doc.GetText(textName),
		aw:    awareness.New(uint64(doc.ClientID())),
		conns: make(map[*conn]map[uint64]struct{}),
	}
	r.saveWorker = newSerializedDebounceIntervalWorker(func() {
		if r.closed.Load() {
			return
		}
		c.mutexes.lock(name)
		c.store.Write(name, r.doc.EncodeStateAsUpdate())
		c.mutexes.unlock(name)
	}, 5*time.Second, c.settings.SaveInterval)
	doc.OnUpdate(func(update []byte, _ any) {
		r.broadcast(syncFrame(ysync.EncodeUpdate(update)))
		r.saveWorker.do()
	})
	r.aw.OnChange(func(evt awareness.ChangeEvent) {
		if cn, ok := evt.Origin.(*conn); ok {
			r.mu.Lock()
			if ids, active := r.conns[cn]; active {
				for _, id := range evt.Added {
					ids[id] = struct{}{}
				}
				for _, id := range evt.Removed {
					delete(ids, id)
				}
			}
			r.mu.Unlock()
		}
		var changed = make([]uint64, 0, len(evt.Added)+len(evt.Updated)+len(evt.Removed))
		changed = append(changed, evt.Added...)
		changed = append(changed, evt.Updated...)
		changed = append(changed, evt.Removed...)
		if len(changed) > 0 {
			r.broadcast(awarenessFrame(r.aw.EncodeUpdate(changed)))
		}
	})
	r.aw.StartAutoExpiry(awarenessExpiryTimeout)
	return r
}

func (r *room) broadcast(frame []byte) {
	r.mu.Lock()
	var conns = make([]*conn, 0, len(r.conns))
	for cn := range r.conns {
		conns = append(conns, cn)
	}
	r.mu.Unlock()
	for _, cn := range conns {
		cn.send(frame)
	}
}

func (r *room) processFrame(data []byte, cn *conn) error {
	var err error
	var dec = encoding.NewDecoder(data)
	var tag uint64
	if tag, err = dec.ReadVarUint(); err != nil {
		return fmt.Errorf("collab: read message tag: %w", err)
	}
	switch tag {
	case messageSync:
		var payload = dec.RemainingBytes()
		var reply, err = ysync.ApplySyncMessage(r.doc, payload, cn)
		if err != nil {
			r.c.log.Warn("collab: apply sync", "error", err)
		}
		if len(reply) > 0 {
			cn.send(syncFrame(reply))
		}
	case messageAwareness:
		var payload, err = dec.ReadVarBytes()
		if err != nil {
			return fmt.Errorf("collab: read awareness payload: %w", err)
		}
		if err = r.aw.ApplyUpdate(payload, cn); err != nil {
			r.c.log.Warn("collab: apply awareness", "error", err)
		}
	case messageQueryAwareness:
		cn.send(awarenessFrame(r.aw.EncodeUpdate(nil)))
	case messageAuth:
	default:
		r.c.log.Warn("collab: unknown message type", "type", tag)
	}
	return nil
}

func (r *room) join(cn *conn) (joined bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed.Load() {
		return false
	}
	r.wgReaders.Add(1)
	r.conns[cn] = make(map[uint64]struct{})
	return true
}

func (r *room) leave(cn *conn) {
	r.mu.Lock()
	var ids, ok = r.conns[cn]
	delete(r.conns, cn)
	var empty = len(r.conns) == 0
	r.mu.Unlock()
	if !ok {
		return
	}
	if r.c.cb.OnCloseConnection != nil {
		r.c.cb.OnCloseConnection(r.name, cn.req)
	}
	r.removeAwareness(ids)
	if empty {
		r.close()
	}
}

func (r *room) close() {
	if !r.closed.CompareAndSwap(false, true) {
		return
	}
	r.c.wgclose.Go(func() {
		r.saveWorker.close()
		r.c.mutexes.lock(r.name)
		defer r.c.mutexes.unlock(r.name)
		r.mu.Lock()
		var conns = make([]*conn, 0, len(r.conns))
		for cn := range r.conns {
			conns = append(conns, cn)
		}
		r.mu.Unlock()
		for _, cn := range conns {
			cn.close(websocket.StatusGoingAway, "")
		}
		r.wgReaders.Wait()
		if r.c.cb.OnCloseRoom != nil {
			r.c.cb.OnCloseRoom(r.name)
		}
		r.c.removeRoom(r.name, r)
		r.aw.Destroy()
		var content = r.text.ToString()
		var err = r.c.doc.Save(context.Background(), r.name, content)
		if err == nil {
			r.c.store.Delete(r.name)
			return
		}
		r.c.store.Write(r.name, r.doc.EncodeStateAsUpdate())
	})
}

func (r *room) removeAwareness(ids map[uint64]struct{}) {
	if len(ids) == 0 {
		return
	}
	type leaving struct {
		id    uint64
		clock uint64
	}
	var known = make([]leaving, 0, len(ids))
	for id := range ids {
		var meta, ok = r.aw.Meta(id)
		if ok {
			known = append(known, leaving{id: id, clock: meta.Clock})
		}
	}
	if len(known) == 0 {
		return
	}
	var enc = encoding.NewEncoder()
	enc.WriteVarUint(uint64(len(known)))
	for _, entry := range known {
		enc.WriteVarUint(entry.id)
		enc.WriteVarUint(entry.clock)
		enc.WriteVarString("null")
	}
	if err := r.aw.ApplyUpdate(enc.Bytes(), nil); err != nil {
		r.c.log.Error("remove awareness", "error", err, "name", r.name)
	}
}

func syncFrame(msg []byte) []byte {
	var enc = encoding.NewEncoder()
	enc.WriteVarUint(messageSync)
	enc.WriteRaw(msg)
	return enc.Bytes()
}

func awarenessFrame(update []byte) []byte {
	var enc = encoding.NewEncoder()
	enc.WriteVarUint(messageAwareness)
	enc.WriteVarBytes(update)
	return enc.Bytes()
}
