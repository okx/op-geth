package streamer

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// Client status
	Active WsClientStatus = iota + 1
	Stopped
	Killed                 WsClientStatus = 0xff
	ClientMessageQueueSize                = 20_000
)

// ClientStatus contains the status of the client websocket connection
type WsClientStatus uint64

// Client contains the state and metadata of the client websocket connection
type WsClient struct {
	conn      net.Conn
	id        string
	heartbeat atomic.Int64
	status    WsClientStatus
	mu        sync.Mutex
	msgQueue  chan []byte
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewWsClient(conn net.Conn, id string) *WsClient {
	ctx, cancel := context.WithCancel(context.Background())
	c := &WsClient{
		id:       id,
		conn:     conn,
		msgQueue: make(chan []byte, ClientMessageQueueSize),
		ctx:      ctx,
		cancel:   cancel,
	}
	c.heartbeat.Store(time.Now().UnixNano())
	return c
}

func (c *WsClient) SetHeartbeat() {
	c.heartbeat.Store(time.Now().UnixNano())
}

// Close shuts down the client gracefully
func (c *WsClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status != Killed {
		c.status = Killed
		c.cancel() // Signal worker to stop
		close(c.msgQueue)
		if c.conn != nil {
			c.conn.Close()
		}
	}
}

// SendMessage enqueues a message for this client (non-blocking)
func (c *WsClient) SendMessage(msg []byte) bool {
	select {
	case c.msgQueue <- msg:
		return true
	default:
		return false
	}
}
