package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"time"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var ErrOffline = errors.New("daemon offline")

type Client struct {
	onAsk    func(Frame)
	onVerify func(error)

	mu   sync.Mutex
	conn net.Conn
	enc  *json.Encoder
	runs map[string]chan Frame
}

func Connect(ctx context.Context, onAsk func(Frame), onVerify func(error), onState func(connected bool)) *Client {
	c := &Client{
		onAsk:    onAsk,
		onVerify: onVerify,
		runs:     map[string]chan Frame{},
	}
	go c.loop(ctx, onState)
	return c
}

func (c *Client) loop(ctx context.Context, onState func(connected bool)) {
	go func() {
		<-ctx.Done()
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
		}
		c.mu.Unlock()
	}()

	for ctx.Err() == nil {
		var dialer net.Dialer
		conn, err := dialer.DialContext(ctx, "unix", filesystem.DaemonSocketPath)
		if err == nil {
			c.mu.Lock()
			c.conn, c.enc = conn, json.NewEncoder(conn)
			c.mu.Unlock()
			onState(true)
			c.read(conn)
			c.detach(conn)
			onState(false)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (c *Client) read(conn net.Conn) {
	dec := json.NewDecoder(conn)
	for {
		var f Frame
		if err := dec.Decode(&f); err != nil {
			return
		}
		switch f.Type {
		case FrameEvent, FrameDone, FrameMCP:
			last := f.Type != FrameEvent
			c.mu.Lock()
			ch := c.runs[f.UUID]
			if last {
				delete(c.runs, f.UUID)
			}
			c.mu.Unlock()
			if ch == nil {
				continue
			}
			ch <- f
			if last {
				close(ch)
			}
		case FrameVerify:
			var err error
			if f.Error != "" {
				err = errors.New(f.Error)
			}
			c.onVerify(err)
		case FrameAsk:
			if f.Ask == nil {
				continue
			}
			c.onAsk(f)
		}
	}
}

func (c *Client) detach(conn net.Conn) {
	conn.Close()
	c.mu.Lock()
	c.conn, c.enc = nil, nil
	runs := c.runs
	c.runs = map[string]chan Frame{}
	c.mu.Unlock()

	for uuid, ch := range runs {
		ch <- Frame{Type: FrameDone, UUID: uuid, Error: ErrOffline.Error()}
		close(ch)
	}
}

func (c *Client) send(f Frame) error {
	if c.enc == nil {
		return ErrOffline
	}
	if err := c.enc.Encode(f); err != nil {
		c.conn.Close()
		return utilsmsg.Wrap(utilsmsg.Unavailable, "Encode", "", err)
	}
	return nil
}

func (c *Client) Run(f Frame) (<-chan Frame, error) {
	ch := make(chan Frame, 64)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.runs[f.UUID] = ch
	if err := c.send(f); err != nil {
		delete(c.runs, f.UUID)
		return nil, err
	}
	return ch, nil
}

func (c *Client) Cancel(taskHash string, pause bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.send(Frame{Type: FrameCancel, TaskHash: taskHash, Pause: pause})
}

func (c *Client) Steer(sessionID, windowHash, text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.send(Frame{Type: FrameSteer, SessionID: sessionID, Rayload: &Payload{Input: text, WindowHash: windowHash}})
}

func (c *Client) Reply(id string, r runtime.Reply, password string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.send(Frame{Type: FrameReply, Reply: &Reply{
		ID:        id,
		Approve:   r.Approve,
		Remember:  r.Remember,
		AllowTurn: r.AllowTurn,
		Skip:      r.Skip,
		Abort:     r.Error != nil,
		Reason:    r.Reason,
		Answers:   r.Answers,
		Password:  password,
	}})
}

func (c *Client) MCP(action, server string) (*MCP, error) {
	frames, err := c.Run(Frame{Type: FrameMCP, UUID: go_pkg_utils.UUID(), MCP: &MCP{Action: action, Server: server}})
	if err != nil {
		return nil, err
	}
	f := <-frames
	if f.Error != "" {
		return nil, errors.New(f.Error)
	}
	if f.MCP == nil {
		return &MCP{}, nil
	}
	return f.MCP, nil
}
