package collab

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/KrainovSD/go-packages/helpers"
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
	conns      *conns
	mu         sync.Mutex
	closed     bool
	dirty      atomic.Bool
	wgReaders  sync.WaitGroup
}

func newRoom(c *Collab, name string, document string, snapshot []byte) *room {
	var doc = crdt.New(crdt.WithGC(true), crdt.WithClientID(crdt.NewClientID()))
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
		conns: newConns(),
	}
	r.saveWorker = newSerializedDebounceIntervalWorker(func() {
		r.mu.Lock()
		if r.closed || !r.dirty.Swap(false) {
			r.mu.Unlock()
			return
		}
		r.mu.Unlock()
		c.mutexes.lock(name)
		defer c.mutexes.unlock(name)
		c.store.Write(name, r.doc.EncodeStateAsUpdate())
	}, 5*time.Second, c.settings.SaveInterval)
	doc.OnUpdate(func(update []byte, origin any) {
		r.broadcast(syncFrame(ysync.EncodeUpdate(update)))
		r.dirty.Store(true)
		r.saveWorker.do()
	})
	r.aw.OnUpdate(func(evt awareness.UpdateEvent) {
		r.conns.awarenessUpdate(evt)
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

func (r *room) join(cn *conn) (joined bool) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return false
	}
	r.wgReaders.Add(1)
	r.conns.set(cn)
	r.mu.Unlock()
	return true
}

func (r *room) leave(cn *conn) {
	var ids, empty, ok = r.conns.delete(cn)
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
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	r.mu.Unlock()
	r.c.wgRoomClose.Go(func() {
		r.saveWorker.close()
		r.c.mutexes.lock(r.name)
		defer r.c.mutexes.unlock(r.name)
		r.c.rooms.delete(r.name, r)
		var conns = r.conns.val()
		helpers.AsyncWork(helpers.GetMaxThreads(), len(conns), func(i int) error {
			conns[i].close(websocket.StatusGoingAway, "")
			return nil
		})
		r.wgReaders.Wait()
		if r.c.cb.OnCloseRoom != nil {
			r.c.cb.OnCloseRoom(r.name)
		}
		r.aw.Destroy()
		var err = r.c.doc.Save(context.Background(), r.name, r.text.ToString())
		if err == nil {
			r.c.store.Delete(r.name)
			return
		}
		r.c.store.Write(r.name, r.doc.EncodeStateAsUpdate())
	})
}

func (r *room) broadcast(frame []byte) {
	for _, cn := range r.conns.val() {
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
		var reply, err = r.applyPatch(payload, cn)
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

func (r *room) applyPatch(patch []byte, origin any) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, fmt.Errorf("collab: room %s is closed", r.name)
	}
	var reply []byte
	var err error
	if reply, err = ysync.ApplySyncMessage(r.doc, patch, origin); err != nil {
		return nil, fmt.Errorf("collab: apply sync message: %w", err)
	}
	return reply, nil
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

type conns struct {
	mu  sync.RWMutex
	raw map[*conn]map[uint64]struct{}
}

func newConns() *conns {
	var r = &conns{raw: make(map[*conn]map[uint64]struct{}, 64)}
	return r
}

func (c *conns) awarenessUpdate(evt awareness.UpdateEvent) {
	if cn, ok := evt.Origin.(*conn); ok {
		c.mu.Lock()
		if ids, active := c.raw[cn]; active {
			for _, id := range evt.Added {
				ids[id] = struct{}{}
			}
			for _, id := range evt.Removed {
				delete(ids, id)
			}
		}
		c.mu.Unlock()
	}
}

func (c *conns) val() []*conn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var conns = make([]*conn, 0, len(c.raw))
	for cn := range c.raw {
		conns = append(conns, cn)
	}
	return conns
}

func (c *conns) set(cn *conn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.raw[cn] = make(map[uint64]struct{})
}

func (c *conns) delete(cn *conn) (ids map[uint64]struct{}, empty bool, ok bool) {
	c.mu.Lock()
	ids, ok = c.raw[cn]
	delete(c.raw, cn)
	empty = len(c.raw) == 0
	c.mu.Unlock()
	return ids, empty, ok
}
