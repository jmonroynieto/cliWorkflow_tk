package rpc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Client is a line-delimited JSON client over a single connection.
type Client struct {
	conn   net.Conn
	sc     *bufio.Scanner
	w      *bufio.Writer
	mu     sync.Mutex
	nextID atomic.Int64
}

// Dial connects to the Unix socket at path with a short timeout.
func Dial(ctx context.Context, socketPath string) (*Client, error) {
	var d net.Dialer
	if deadline, ok := ctx.Deadline(); ok {
		d.Timeout = time.Until(deadline)
	} else {
		d.Timeout = 300 * time.Millisecond
	}
	conn, err := d.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	return &Client{
		conn: conn,
		sc:   sc,
		w:    bufio.NewWriter(conn),
	}, nil
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.Close()
}

// Call sends a method with params and decodes the result into resultOut (may be nil).
func (c *Client) Call(ctx context.Context, method string, params any, resultOut any) error {
	var rawParams json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		rawParams = b
	}
	req := Request{
		V:      ProtocolVersion,
		ID:     c.nextID.Add(1),
		Method: method,
		Params: rawParams,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if deadline, ok := ctx.Deadline(); ok {
		_ = c.conn.SetDeadline(deadline)
		defer c.conn.SetDeadline(time.Time{})
	}

	if _, err := c.w.Write(body); err != nil {
		return fmt.Errorf("write request: %w", err)
	}
	if err := c.w.WriteByte('\n'); err != nil {
		return err
	}
	if err := c.w.Flush(); err != nil {
		return err
	}

	if !c.sc.Scan() {
		if err := c.sc.Err(); err != nil {
			return fmt.Errorf("read response: %w", err)
		}
		return fmt.Errorf("connection closed")
	}
	var resp Response
	if err := json.Unmarshal(c.sc.Bytes(), &resp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if resp.ID != req.ID {
		return fmt.Errorf("response id mismatch: got %d want %d", resp.ID, req.ID)
	}
	if !resp.OK {
		if resp.Error == "" {
			return fmt.Errorf("rpc %s failed", method)
		}
		return fmt.Errorf("%s", resp.Error)
	}
	if resultOut == nil || len(resp.Result) == 0 {
		return nil
	}
	return json.Unmarshal(resp.Result, resultOut)
}
