package collab

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/KrainovSD/go-packages/helpers"
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
