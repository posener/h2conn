package h2conn

import (
	"bufio"
	"context"
	"io"
	"log"
)

// Conn is client/server symmetric connection.
// It implements the io.Reader/io.Writer/io.Closer to read/write or close the connection to the other side.
// It also has a Send/Recv function to use channels to communicate with the other side.
type Conn struct {
	r   io.Reader
	wc  io.WriteCloser
	in  chan []byte
	out chan []byte
}

func newConn(ctx context.Context, r io.Reader, wc io.WriteCloser) *Conn {
	in := make(chan []byte)
	out := make(chan []byte)
	c := &Conn{
		r:   r,
		wc:  wc,
		in:  in,
		out: out,
	}

	c.loop(ctx)
	return c
}

// Send returns a send channel - messages that are sent into this channel will be sent to the other side.
func (c *Conn) Send() chan<- []byte {
	return c.out
}

// Recv returns a receive channel - messages that were sent from the other side will be received here.
func (c *Conn) Recv() <-chan []byte {
	return c.in
}

func (c *Conn) Write(data []byte) (int, error) {
	n := len(data)
	_, err := c.wc.Write(encode(data))
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (c *Conn) Read(data []byte) (int, error) {
	resp, ok := <-c.in
	if !ok {
		return 0, io.EOF
	}
	copy(data, resp)
	return len(resp), nil
}

func (c *Conn) Close() error {
	return c.wc.Close()
}

func (c *Conn) loop(ctx context.Context) {
	go c.readLoop(ctx)
	go c.writeLoop(ctx)
}

func (c *Conn) readLoop(ctx context.Context) {
	// close the in channel when read loop finishes
	defer close(c.in)

	// buffer to enable read lines from the connection
	buf := bufio.NewReader(c.r)

	for ctx.Err() == nil {
		line, prefix, err := buf.ReadLine()
		if err != nil {
			// TODO: store read error in the conn struct?
			log.Printf("Failed read: %s", err)
			return
		}
		if prefix {
			// TODO: support long lines
			log.Printf("Long lines not supported")
			return
		}
		line = decode(line)
		var cp = make([]byte, len(line))
		copy(cp, line)
		c.in <- cp
	}
}

func (c *Conn) writeLoop(ctx context.Context) {
	for {
		select {
		case msg, ok := <-c.out:
			if !ok {
				return
			}
			c.Write(msg)
		case <-ctx.Done():
			return
		}
	}
}
