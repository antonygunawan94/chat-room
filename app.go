package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/antony/chat-room/message"
)

var addr = flag.String("addr", ":8080", "http service address")
var parser = message.NewMessageParser()

func main() {
	flag.Parse()
	hub := NewHub()
	go hub.run()
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ServeWs(w, r)
	})

	//File server
	fs := http.FileServer(http.Dir("public"))
	http.Handle("/public/", http.StripPrefix("/public/", fs))

	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
