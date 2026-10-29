package collab

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
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
	rooms       map[string]*room
	wgRoomClose sync.WaitGroup
	closed      atomic.Bool
}

func New(opts *Options) (*Collab, error) {
	if opts == nil {
		return nil, fmt.Errorf("collab: options must be passed")
	}
	if err := opts.validate(); err != nil {
		return nil, err
	}
	opts.newDefault()
	var c = &Collab{
		doc:      opts.Doc,
		store:    opts.Store,
		log:      opts.Log,
		settings: &opts.Settings,
		cb:       &opts.Callbacks,
		mutexes:  newMutexStore(),
		rooms:    make(map[string]*room, 1024),
	}
	return c, nil
}

func (c *Collab) Close(ctx context.Context) {
	if !c.closed.CompareAndSwap(false, true) {
		return
	}
	c.mu.Lock()
	var rooms = make([]*room, 0, len(c.rooms))
	for _, rm := range c.rooms {
		rooms = append(rooms, rm)
	}
	c.mu.Unlock()
	var done = make(chan struct{})
	go func() {
		helpers.AsyncWork(helpers.GetMaxThreads(), len(rooms), func(i int) error {
			rooms[i].close()
			return nil
		})
		c.wgRoomClose.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}

}

func (c *Collab) removeRoom(name string, rm *room) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rooms[name] == rm {
		delete(c.rooms, name)
	}
}
