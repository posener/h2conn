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
	"time"

	"github.com/marcusolsson/tui-go"
	"github.com/posener/h2conn"
	"github.com/posener/h2conn/example/chat"
	"golang.org/x/net/http2"
)

const url = "https://localhost:8000"

func main() {
	client, resp, err := h2conn.Dial(context.Background(), url, h2conn.OptTransport(&http2.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}))
	if err != nil {
		log.Fatalf("Initiate client: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Bad status code: %d", resp.StatusCode)
	}

	fmt.Print("Name: ")
	nameReader := bufio.NewReader(os.Stdin)
	name, _ := nameReader.ReadString('\n')
	client.Send() <- []byte(name)
	loginResp := <-client.Recv()
	if string(loginResp) != "ok" {
		log.Fatalf("Failed login: %s", string(loginResp))
	}

	// closing the login will logout
	defer client.Close()

	history := tui.NewVBox()

	historyScroll := tui.NewScrollArea(history)
	historyScroll.SetAutoscrollToBottom(true)

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
			return
		}
		body, err := json.Marshal(chat.Post{
			Message: e.Text(),
			Time:    time.Now(),
		})
		if err != nil {
			log.Fatalf("Failed marshalling message: %s", err)
		}
		client.Write(body)
		input.SetText("")
	})

	ui, err := tui.New(root)
	if err != nil {
		log.Fatal(err)
	}

	ui.SetKeybinding("Esc", func() { ui.Quit() })

	go func() {
		for line := range client.Recv() {
			var post chat.Post
			err = json.Unmarshal(line, &post)
			if err != nil {
				log.Fatalf("Failed unmarshaling %s %s", string(line), err)
			}
			history.Append(tui.NewHBox(
				tui.NewLabel(post.Time.String()),
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
