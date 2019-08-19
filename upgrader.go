package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

// UpgraderUpgrade returns the websocket connection on successful upgrade
// to websocket and nil otherwise.
func UpgraderUpgrade(w http.ResponseWriter, r *http.Request) *websocket.Conn {
	if r.Header.Get("Sec-WebSocket-Protocol") != SpecWebSocketProto {
		log.Print("UpgraderUpgrade: missing websocket protocol")
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	h := http.Header{}
	h.Add("Sec-WebSocket-Protocol", SpecWebSocketProto)
	var u websocket.Upgrader
	conn, err := u.Upgrade(w, r, h)
	if err != nil {
		log.Printf("UpgraderUpgrade: u.Upgrade: %s", err.Error())
		return nil
	}
	return conn
}
