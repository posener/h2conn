package main

import (
	"bytes"
	"log"
	"net/http"

	"github.com/posener/h2conn"
)

func main() {
	srv := &http.Server{Addr: ":8000", Handler: &server{}}
	log.Printf("Serving on http://0.0.0.0:8000")
	log.Fatal(srv.ListenAndServeTLS("server.crt", "server.key"))
}

type server struct{}

func (c *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h2conn.New(w, r)
	if err != nil {
		log.Printf("Failed creating connection from %s: %s", r.RemoteAddr, err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	log.Printf("[%s] Joined", r.RemoteAddr)
	defer log.Printf("[%s] Left", r.RemoteAddr)
	for msg := range conn.Receive {
		log.Printf("[%s] Sent: %s", r.RemoteAddr, string(msg))
		conn.Send <- bytes.ToUpper(msg)
	}
}
