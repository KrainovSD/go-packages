package collab

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/KrainovSD/go-packages/helpers"
	"github.com/reearth/ygo/crdt"
	"github.com/reearth/ygo/encoding"
	ysync "github.com/reearth/ygo/sync"
)

type Callbacks struct {
	OnOpenRoom        OnOpenRoom
	OnCloseRoom       OnCloseRoom
	OnOpenConnection  OnOpenConnection
	OnCloseConnection OnCloseConnection
}

type Settings struct {
	Origins      []string
	SaveInterval time.Duration
	PingInterval time.Duration
	PingTimeout  time.Duration
	ReadLimit    int64
}

type Options struct {
	Doc   DocumentService
	Store StateStore
	Log   *slog.Logger
	Callbacks
	Settings
}

func (o *Options) validate() error {
	if o.Doc == nil {
		return fmt.Errorf("collab: doc must be passed")
	}
	if o.Store == nil {
		return fmt.Errorf("collab: store must be passed")
	}
	if o.Log == nil {
		return fmt.Errorf("collab: log must be passed")
	}
	if o.SaveInterval < 0 {
		return fmt.Errorf("collab: save interval must not be negative")
	}
	if o.PingInterval < 0 {
		return fmt.Errorf("collab: ping interval must not be negative")
	}
	if o.PingTimeout < 0 {
		return fmt.Errorf("collab: ping timeout must not be negative")
	}
	return nil
}

func (o *Options) newDefault() {
	if o.SaveInterval == 0 {
		o.SaveInterval = time.Minute
	}
	if o.PingInterval == 0 {
		o.PingInterval = 30 * time.Second
	}
	if o.PingTimeout == 0 {
		o.PingTimeout = 15 * time.Second
	}
	if o.ReadLimit == 0 {
		o.ReadLimit = 10 << 20
	}
}

type Collab struct {
	doc         DocumentService
	store       StateStore
	log         *slog.Logger
	settings    *Settings
	cb          *Callbacks
	mutexes     *mutexStore
	mu          sync.RWMutex
	rooms       *rooms
	wgRoomClose sync.WaitGroup
	closed      bool
}

func New(optsPtr *Options) (*Collab, error) {
	if optsPtr == nil {
		return nil, fmt.Errorf("collab: options must be passed")
	}
	var opts = *optsPtr
	if err := opts.validate(); err != nil {
		return nil, err
	}
	opts.newDefault()
	return &Collab{
		doc:      opts.Doc,
		store:    opts.Store,
		log:      opts.Log,
		settings: &opts.Settings,
		cb:       &opts.Callbacks,
		mutexes:  newMutexStore(),
		rooms:    newRooms(),
	}, nil
}

func (c *Collab) Close(ctx context.Context) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.mu.Unlock()
	var rooms = c.rooms.val()
	helpers.LimitWork(ctx, 0, func(ctx context.Context) error {
		helpers.AsyncWork(helpers.GetMaxThreads(), len(rooms), func(i int) error {
			rooms[i].close()
			return nil
		})
		c.wgRoomClose.Wait()
		return nil
	})
}

func (c *Collab) GetRoomsByUserId(userId string) []string {
	var found []string
	for _, rm := range c.rooms.val() {
		for _, cn := range rm.conns.val() {
			if cn.userId == userId {
				found = append(found, rm.name)
				break
			}
		}
	}
	return found
}

func (c *Collab) GetRooms() []string {
	var found []string
	for _, rm := range c.rooms.val() {
		found = append(found, rm.name)
	}
	return found
}

func (c *Collab) HasRoom(roomId string) bool {
	return c.rooms.get(roomId) != nil
}

func (c *Collab) GetContent(roomId string) []byte {
	var rm = c.rooms.get(roomId)
	if rm == nil {
		return nil
	}
	return []byte(rm.text.ToString())
}

func (c *Collab) SyncDoc(roomId string, doc *crdt.Doc) error {
	if doc == nil {
		return fmt.Errorf("collab: doc must be passed")
	}
	var rm = c.rooms.get(roomId)
	if rm == nil {
		return fmt.Errorf("collab: room %s does not exist or has been closed", roomId)
	}
	var clientStep1 = ysync.EncodeSyncStep1(doc)
	var serverStep2, err = ysync.EncodeSyncStep2(rm.doc, clientStep1)
	if err != nil {
		return fmt.Errorf("collab: room %s encode server step2: %w", roomId, err)
	}
	if _, err = ysync.ApplySyncMessage(doc, serverStep2, nil); err != nil {
		return fmt.Errorf("collab: room %s apply server step2: %w", roomId, err)
	}
	var serverStep1 = ysync.EncodeSyncStep1(rm.doc)
	var clientStep2 []byte
	if clientStep2, err = ysync.EncodeSyncStep2(doc, serverStep1); err != nil {
		return fmt.Errorf("collab: room %s encode client step2: %w", roomId, err)
	}
	if _, err = rm.applyPatch(clientStep2, nil); err != nil {
		return fmt.Errorf("collab: room %s apply client step2: %w", roomId, err)
	}
	return nil
}

func (c *Collab) ApplyPatch(roomId string, patch []byte) ([]byte, error) {
	var rm = c.rooms.get(roomId)
	if rm == nil {
		return nil, fmt.Errorf("collab: room %s does not exist or has been closed", roomId)
	}
	var dec = encoding.NewDecoder(patch)
	var tag uint64
	var err error
	if tag, err = dec.ReadVarUint(); err != nil {
		return nil, fmt.Errorf("collab: read patch message tag: %w", err)
	}
	if tag != messageSync {
		return nil, fmt.Errorf("collab: patch is not a sync frame")
	}
	var payload = dec.RemainingBytes()
	var decInner = encoding.NewDecoder(payload)
	var tagInner uint64
	if tagInner, err = decInner.ReadVarUint(); err != nil {
		return nil, fmt.Errorf("collab: read patch sync message type: %w", err)
	}
	if tagInner == uint64(ysync.MsgSyncStep1) || tagInner == uint64(ysync.MsgSyncStep2) {
		return nil, fmt.Errorf("collab: patch must be an update message, not sync step1 or step2")
	}
	return rm.applyPatch(payload, nil)
}

func (c *Collab) LockRoom(roomId string) {
	c.mutexes.lock(roomId)
}

func (c *Collab) UnlockRoom(roomId string) error {
	if !c.mutexes.tryUnlock(roomId) {
		return fmt.Errorf("collab: mutex not exists for room %s", roomId)
	}
	return nil
}

type rooms struct {
	mu  sync.RWMutex
	raw map[string]*room
}

func newRooms() *rooms {
	var r = &rooms{raw: make(map[string]*room, 1024)}
	return r
}

func (r *rooms) val() []*room {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var rooms = make([]*room, 0, len(r.raw))
	for _, room := range r.raw {
		rooms = append(rooms, room)
	}
	return rooms
}

func (r *rooms) get(name string) *room {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var room, ok = r.raw[name]
	if !ok {
		return nil
	}
	return room
}

func (r *rooms) set(name string, rm *room) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.raw[name] = rm
}

func (r *rooms) delete(name string, rm *room) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var current, ok = r.raw[name]
	if !ok || current != rm {
		return
	}
	delete(r.raw, name)
}
