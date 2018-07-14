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
	go func() {
		buf := bufio.NewReader(c.r)
		defer close(c.in)
		for ctx.Err() == nil {
			line, prefix, err := buf.ReadLine()
			if err != nil {
				log.Printf("Failed read: %s", err)
				return
			}
			// TODO: treat long lines
			if prefix {
				return
			}
			line = decode(line)
			var cp = make([]byte, len(line))
			copy(cp, line)
			c.in <- cp
		}
	}()

	go func() {
		for {
			select {
			case msg := <-c.out:
				c.Write(msg)
			case <-ctx.Done():
				return
			}
		}
	}()
}
