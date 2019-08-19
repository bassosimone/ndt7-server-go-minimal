package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type receiver struct {
	begin time.Time
	conn  *websocket.Conn
	mutex sync.Mutex
	ri    ReceiverInfo
}

func (r *receiver) receiverLoop(ctx context.Context) {
	var count int64
	r.conn.SetReadLimit(SpecMaxBinaryMessageSize)
	for ctx.Err() == nil {
		_, mdata, err := r.conn.ReadMessage()
		if err != nil {
			log.Printf("receiver.receiverLoop: conn.ReadMessage: %s", err.Error())
			return
		}
		count += int64(len(mdata))
		r.mutex.Lock()
		r.ri.ElapsedSeconds = time.Now().Sub(r.begin).Seconds()
		r.ri.NumBytes = count
		r.mutex.Unlock()
	}
}

func (r *receiver) senderLoop(ctx context.Context) {
	for ctx.Err() == nil {
		time.Sleep(250 * time.Millisecond)
		var ri ReceiverInfo
		r.mutex.Lock()
		ri = r.ri
		r.mutex.Unlock()
		//log.Printf("receiver.senderLoop: %f %d", ri.ElapsedSeconds, ri.NumBytes)
		err := r.conn.WriteJSON(ri)
		if err != nil {
			log.Printf("receiver.senderLoop: conn.WriteJSON: %s", err.Error())
			return
		}
	}
}

func receiverMain(ctx context.Context, conn *websocket.Conn) {
	defer conn.Close()
	const timeout = 15 * time.Second
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		log.Printf("receiverMain: conn.SetReadDeadline: %s", err.Error())
		return
	}
	if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		log.Printf("receiverMain: conn.SetWriteDeadline: %s", err.Error())
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	r := &receiver{
		begin: time.Now(),
		conn:  conn,
	}
	go r.senderLoop(ctx)
	r.receiverLoop(ctx)
}
