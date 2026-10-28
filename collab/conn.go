package collab

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

const frameQueueSize = 64

type conn struct {
	ws     *websocket.Conn
	room   *room
	req    *http.Request
	out    chan []byte
	closed atomic.Bool
	done   chan struct{}
}

func newConn(ws *websocket.Conn, room *room, req *http.Request) *conn {
	return &conn{
		ws:   ws,
		room: room,
		req:  req,
		out:  make(chan []byte, frameQueueSize),
		done: make(chan struct{}),
	}
}

func (cn *conn) send(frame []byte) bool {
	select {
	case cn.out <- frame:
		return true
	default:
		cn.close(websocket.StatusMessageTooBig, "slow consumer")
		return false
	}
}

func (cn *conn) close(status websocket.StatusCode, msg string) {
	if cn.closed.CompareAndSwap(false, true) {
		close(cn.done)
		var err = cn.ws.Close(status, msg)
		if err != nil && !errors.Is(err, net.ErrClosed) {
			cn.room.c.log.Warn("close connection", "error", err, "name", cn.room.name)
		}
		cn.room.leave(cn)
	}
}

func (cn *conn) writeLoop() {
	for {
		select {
		case frame := <-cn.out:
			var ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
			var err = cn.ws.Write(ctx, websocket.MessageBinary, frame)
			cancel()
			if err != nil {
				cn.close(websocket.StatusNormalClosure, "")
				return
			}
		case <-cn.done:
			cn.close(websocket.StatusNormalClosure, "")
			return
		}
	}
}

func (cn *conn) pingLoop(interval time.Duration, timeout time.Duration) {
	var ticker = time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			var ctx, cancel = context.WithTimeout(context.Background(), timeout)
			var err = cn.ws.Ping(ctx)
			cancel()
			if err != nil {
				cn.close(websocket.StatusNormalClosure, "")
				return
			}
		case <-cn.done:
			cn.close(websocket.StatusNormalClosure, "")
			return
		}
	}
}
