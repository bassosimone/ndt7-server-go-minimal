package main

import (
	"context"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// receiverLoop receives binary messages and sends feedback.
func receiverLoop(ctx context.Context, conn *websocket.Conn) {
	begin := time.Now()
	var count int64
	previous := begin
	conn.SetReadLimit(SpecMaxMessageSize)
	for ctx.Err() == nil {
		_, mdata, err := conn.ReadMessage()
		if err != nil {
			log.Printf("receiverLoop: conn.ReadMessage: %s", err.Error())
			return
		}
		count += int64(len(mdata))
		now := time.Now()
		const measurementInterval = 250 * time.Millisecond
		if now.Sub(previous) < measurementInterval {
			continue
		}
		ri := ReceiverInfo{
			ElapsedSeconds: now.Sub(previous).Seconds(),
			NumBytes      : count,
		}
		err = conn.WriteJSON(ri)
		if err != nil {
			log.Printf("receiverLoop: conn.WriteJSON: %s", err.Error())
			return
		}
	}
}

// ReceiverMain takes ownership of |conn| and sends data to the peer for 10 s.
func ReceiverMain(ctx context.Context, conn *websocket.Conn) {
	defer conn.Close()
	const timeout = 15 * time.Second
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		log.Printf("Receiver: conn.SetReadDeadline: %s", err.Error())
		return
	}
	if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		log.Printf("Receiver: conn.SetWriteDeadline: %s", err.Error())
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	receiverLoop(ctx, conn)
}
