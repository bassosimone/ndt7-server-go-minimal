package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// senderReceiver receives measurements from the receiver.
func senderReceiver(ctx context.Context, conn *websocket.Conn) <-chan ReceiverInfo {
	const channelBuffer = 40
	ch := make(chan ReceiverInfo, channelBuffer)
	go func(ch chan<- ReceiverInfo) {
		const maxMessageSize = 1 << 10
		conn.SetReadLimit(maxMessageSize)
		defer close(ch)
		for ctx.Err() == nil {
			mtype, mdata, err := conn.ReadMessage()
			if err != nil {
				log.Printf("senderReceiver: conn.ReadMessage: %s", err.Error())
				return
			}
			if mtype != websocket.TextMessage {
				log.Printf("senderReceiver: invalid message type")
				return
			}
			var ri ReceiverInfo
			err = json.Unmarshal(mdata, &ri)
			if err != nil {
				log.Printf("senderReceiver: json.Unmarshal: %s", err.Error())
				return
			}
			ch <- ri
		}
	}(ch)
	return ch
}

// senderETA returns the amount of seconds we have in queue
func senderETA(ri ReceiverInfo, queuedBytes int64) (ETA float64) {
	speedSample := float64(ri.NumBytes) / ri.ElapsedSeconds
	notSentBytes := queuedBytes - ri.NumBytes
	if notSentBytes > 0 {
		ETA = float64(notSentBytes) / speedSample
	}
	return
}

// senderAdjustMessageSize possibly changes the message size.
func senderAdjustMessageSize(ri ReceiverInfo, curSize *int) bool {
	speedSample := float64(ri.NumBytes) / ri.ElapsedSeconds
	const desiredReceiveTimeSecond = 0.25
	desiredSize := speedSample * desiredReceiveTimeSecond
	if desiredSize < float64(*curSize) * 1.2 {
		return false
	}
	if desiredSize > float64(SpecMaxMessageSize) {
		desiredSize = float64(SpecMaxMessageSize)
	}
	*curSize = int(desiredSize)
	return true
}

// senderSender sends binary messages to the receiver.
func senderSender(ctx context.Context, conn *websocket.Conn, ch <-chan ReceiverInfo) {
	var binaryMessage *websocket.PreparedMessage
	curSize := SpecInitialMessageSize
	var queuedBytes int64
	var ETA float64
	for ctx.Err() == nil {
		select {
		case ri, ok := <-ch:
			if !ok {
				log.Print("senderSender: channel closed")
				return
			}
			if ri.ElapsedSeconds <= 0.0 || ri.NumBytes <= 0 {
				log.Print("senderSender: received invalid message")
				return
			}
			ETA = senderETA(ri, queuedBytes)
			if senderAdjustMessageSize(ri, &curSize) {
				binaryMessage = nil
				// FALLTHROUGH
			}
			continue // Process consecutive messages
		default:
			// NOTHING
		}
		if ETA > 10.0 {
			log.Printf("ETA: %f", ETA)
			time.Sleep(250 * time.Millisecond)
			// FALLTHROUGH
		}
		if binaryMessage == nil {
			log.Printf("senderSender: new message size: %d", curSize)
			data := make([]byte, curSize)
			var err error
			binaryMessage, err = websocket.NewPreparedMessage(
				websocket.BinaryMessage, data,
			)
			if err != nil {
				log.Printf("senderSender: websocket.PreparedMesage: %s", err.Error())
				return
			}
		}
		if err := conn.WritePreparedMessage(binaryMessage); err != nil {
			return
		}
		queuedBytes += int64(curSize)
	}
}

// SenderMain takes ownership of |conn| and sends data to the peer for 10 s.
func SenderMain(ctx context.Context, conn *websocket.Conn) {
	defer conn.Close()
	const timeout = 10 * time.Second
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		log.Printf("Sender: conn.SetReadDeadline: %s", err.Error())
		return
	}
	if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		log.Printf("Sender: conn.SetWriteDeadline: %s", err.Error())
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	senderSender(ctx, conn, senderReceiver(ctx, conn))
}
