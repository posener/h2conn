package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/posener/h2conn"
)

const url = "https://localhost:8000"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go catchSignal(cancel)

	client, resp, err := h2conn.Dial(ctx, url, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		log.Fatalf("Initiate client: %s", err)
	}
	if resp.StatusCode != http.StatusCreated {
		log.Fatalf("Bad status code: %d", resp.StatusCode)
	}

	// closing the login will logout
	defer client.Close()

	reader := bufio.NewReader(os.Stdin)

	defer log.Println("Exited")
	for ctx.Err() == nil {
		fmt.Print("Send: ")
		msg, _ := reader.ReadString('\n')

		// send message
		select {
		case client.Send <- []byte(msg):
		case <-ctx.Done():
			return
		}

		// receive message
		select {
		case resp := <-client.Receive:
			fmt.Printf("Got: %s\n", string(resp))
		case <-ctx.Done():
			return
		}
	}
}

func catchSignal(cancel context.CancelFunc) {
	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt)
	<-sig
	log.Println("Cancelling due to interrupt")
	cancel()
}
