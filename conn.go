package h2conn

import (
	"bufio"
	"context"
	"io"
	"log"
)

type Conn struct {
	Send    chan<- []byte
	Receive <-chan []byte
	r       io.Reader
	wc      io.WriteCloser
	in      chan []byte
	out     chan []byte
}

func newConn(ctx context.Context, r io.Reader, wc io.WriteCloser) *Conn {
	in := make(chan []byte)
	out := make(chan []byte)
	c := &Conn{
		r:       r,
		wc:      wc,
		in:      in,
		out:     out,
		Receive: in,
		Send:    out,
	}

	c.loop(ctx)
	return c
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
