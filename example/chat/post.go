package main

import "time"

// Post is a chat post
type Post struct {
	User    string
	Message string
	Time    time.Time
}
