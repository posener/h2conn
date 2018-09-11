package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/posener/h2conn"
)

type encoder interface {
	Encode(interface{}) error
}

type server struct {
	conns map[string]encoder
	lock  sync.RWMutex
}

func main() {
	c := server{conns: make(map[string]encoder)}
	srv := &http.Server{Addr: ":8000", Handler: &c}
	log.Printf("Serving on http://0.0.0.0:8000")
	log.Fatal(srv.ListenAndServeTLS("server.crt", "server.key"))
}

func (c *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h2conn.Accept(w, r)
	if err != nil {
		log.Printf("Failed creating http2 connection: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	var (
		// in and out send and receive GOB messages to the client
		in, out = gob.NewDecoder(conn), gob.NewEncoder(conn)
		// Conn has a RemoteAddr property which helps us identify the client
		log = logger{remoteAddr: r.RemoteAddr}
	)

	// First check user login name
	var name string
	err = in.Decode(&name)
	if err != nil {
		log.Printf("Failed reading login name: %v", err)
		return
	}

	log.Printf("Got login: %s", name)

	err = c.login(name, out)
	if err != nil {
		err = out.Encode(err.Error())
		if err != nil {
			log.Printf("Failed sending login response: %v", err)
		}
		return
	}
	err = out.Encode("ok")
	if err != nil {
		log.Printf("Failed sending login response: %v", err)
		return
	}

	// Send a login message to all the connected clients
	c.systemMessage(fmt.Sprintf("%s logged in", name))

	// defer logout of user
	defer c.logout(name)

	// Defer logout log message
	defer log.Printf("User logout: %s", name)

	// Defer logout message to all connected users
	defer c.systemMessage(fmt.Sprintf("%s logged out", name))

	// wait for client to close connection
	for r.Context().Err() == nil {
		var post Post
		err := in.Decode(&post)
		if err != nil {
			log.Printf("Failed getting post: %v", err)
			return
		}
		log.Printf("Got msg: %+v", post)
		c.post(name, post)
	}
}

func (c *server) login(name string, enc encoder) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if _, ok := c.conns[name]; ok {
		return fmt.Errorf("user already exists")
	}
	c.conns[name] = enc
	return nil
}

func (c *server) logout(name string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.conns, name)
}

func (c *server) post(name string, post Post) {
	post.User = name
	c.lock.RLock()
	defer c.lock.RUnlock()
	var wg sync.WaitGroup
	wg.Add(len(c.conns))
	for dstName, dst := range c.conns {
		go func(dstName string, dst encoder) {
			log.Printf("Writing to %s", dstName)
			err := dst.Encode(&post)
			if err != nil {
				log.Printf("Failed writing to %s: %v", name, err)
			}
			wg.Done()
		}(dstName, dst)
	}
	wg.Wait()
}

func (c *server) systemMessage(message string) {
	c.post("System", Post{Message: message, Time: time.Now()})
}

type logger struct {
	remoteAddr string
}

func (l logger) Printf(format string, args ...interface{}) {
	log.Printf("[%s] %s", l.remoteAddr, fmt.Sprintf(format, args...))
}
