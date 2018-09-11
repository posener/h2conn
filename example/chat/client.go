package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/gob"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/marcusolsson/tui-go"
	"github.com/posener/h2conn"
	"golang.org/x/net/http2"
)

const url = "https://localhost:8000"

func main() {
	ctx := context.Background()

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

	fmt.Print("Name: ")
	nameReader := bufio.NewReader(os.Stdin)
	name, _ := nameReader.ReadString('\n')
	name = strings.TrimRight(name, "\n")

	// in and out send and receive GOB messages to the server
	var in, out = gob.NewDecoder(conn), gob.NewEncoder(conn)

	// Send login request
	err = out.Encode(name)
	if err != nil {
		log.Fatalf("Failed send login name: %v", err)
	}

	// Check login response
	var loginResp string
	err = in.Decode(&loginResp)
	if err != nil {
		log.Fatalf("Failed login: %v", err)
	}
	if loginResp != "ok" {
		log.Fatalf("Failed login: %s", loginResp)
	}

	history := tui.NewVBox()

	historyScroll := tui.NewScrollArea(history)

	historyBox := tui.NewVBox(historyScroll)
	historyBox.SetBorder(true)

	input := tui.NewEntry()
	input.SetFocused(true)
	input.SetSizePolicy(tui.Expanding, tui.Maximum)

	inputBox := tui.NewHBox(input)
	inputBox.SetBorder(true)
	inputBox.SetSizePolicy(tui.Expanding, tui.Maximum)

	root := tui.NewVBox(historyBox, inputBox)
	root.SetSizePolicy(tui.Expanding, tui.Expanding)

	input.OnSubmit(func(e *tui.Entry) {
		if e.Text() == "" {
			return // Skip empty messages
		}
		err := out.Encode(Post{Message: e.Text(), Time: time.Now()})
		if err != nil {
			log.Fatalf("Failed sending message: %v", err)
		}
		input.SetText("")
	})

	ui := tui.New(root)
	ui.SetKeybinding("Esc", func() { ui.Quit() })

	go func() {
		for {
			var post Post
			err = in.Decode(&post)
			if err != nil {
				log.Fatalf("Failed decoding incoming message %v", err)
			}
			history.Append(tui.NewHBox(
				tui.NewLabel(post.Time.Format(time.Kitchen)),
				tui.NewPadder(1, 0, tui.NewLabel(fmt.Sprintf("<%s>", post.User))),
				tui.NewLabel(post.Message),
				tui.NewSpacer(),
			))
		}
	}()

	if err := ui.Run(); err != nil {
		log.Fatal(err)
	}

}
