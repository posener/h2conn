package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/posener/h2conn"
	"github.com/posener/h2conn/example/chat"
)

func main() {
	c := server{conns: make(map[string]*h2conn.Conn)}
	srv := &http.Server{Addr: ":8000", Handler: &c}
	log.Printf("Serving on http://0.0.0.0:8000")
	log.Fatal(srv.ListenAndServeTLS("server.crt", "server.key"))
}

type server struct {
	conns map[string]*h2conn.Conn
	lock  sync.RWMutex
}

func (c *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h2conn.Upgrade(w, r)
	if err != nil {
		log.Printf("Failed creating http2 connection: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	nameBytes := <-conn.Recv()
	name := string(nameBytes)

	log.Printf("Upgrade login: %s", name)

	err = c.login(name, conn)
	if err != nil {
		conn.Write([]byte(err.Error()))
		return
	}
	conn.Write([]byte("ok"))

	// defer logout of user
	defer func() {
		c.lock.Lock()
		delete(c.conns, name)
		c.lock.Unlock()

		log.Printf("User logout: %s", name)
	}()

	// wait for client to close connection
	for {
		select {
		case <-r.Context().Done():
			return
		case req := <-conn.Recv():
			err := c.post(name, req)
			if err != nil {
				log.Printf("Failed posting %q: %s", string(req), err)
				return
			}
		}
	}
}

func (c *server) post(name string, req []byte) error {
	var post chat.Post
	err := json.Unmarshal(req, &post)
	if err != nil {
		return err
	}

	post.User = name

	log.Printf("Got msg: %+v", post)

	msg, err := json.Marshal(post)
	if err != nil {
		return err
	}

	c.lock.RLock()
	defer c.lock.RUnlock()
	for dstName, dst := range c.conns {
		log.Printf("Writing to %s", dstName)
		dst.Write(msg)
	}
	return nil
}

func (c *server) login(name string, conn *h2conn.Conn) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if _, ok := c.conns[name]; ok {
		return fmt.Errorf("user already exists")
	}
	c.conns[name] = conn
	return nil
}
