package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"github.com/posener/h2conn"
	"golang.org/x/net/http2"
)

const url = "https://localhost:8000"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go catchSignal(cancel)

	// We use a client with custom http2.Transport since the server certificate is not signed by
	// an authorized CA, and this is the way to ignore certificate verification errors.
	d := &h2conn.Client{
		Client: &http.Client{
			Transport: &http2.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		},
	}

	conn, resp, err := d.Connect(ctx, url)
	if err != nil {
		log.Fatalf("Initiate conn: %s", err)
	}
	defer conn.Close()

	// Check server status code
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Bad status code: %d", resp.StatusCode)
	}

	var (
		// stdin reads from stdin
		stdin = bufio.NewReader(os.Stdin)

		// in and out send and receive json messages to the server
		in  = json.NewDecoder(conn)
		out = json.NewEncoder(conn)
	)

	defer log.Println("Exited")

	// Loop until user terminates
	fmt.Println("Echo session starts, press ctrl-C to terminate.")
	for ctx.Err() == nil {

		// Ask the user to give a message to send to the server
		fmt.Print("Send: ")
		msg, err := stdin.ReadString('\n')
		if err != nil {
			log.Fatalf("Failed reading stdin: %v", err)
		}
		msg = strings.TrimRight(msg, "\n")

		// Send the message to the server
		err = out.Encode(msg)
		if err != nil {
			log.Fatalf("Failed sending message: %v", err)
		}

		// Receive the response from the server
		var resp string
		err = in.Decode(&resp)
		if err != nil {
			log.Fatalf("Failed receiving message: %v", err)
		}

		fmt.Printf("Got response %q\n", resp)
	}
}

func catchSignal(cancel context.CancelFunc) {
	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt)
	<-sig
	log.Println("Cancelling due to interrupt")
	cancel()
}
