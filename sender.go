package main

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type sender struct {
	conn  *websocket.Conn
	mutex sync.Mutex
	ri    ReceiverInfo
}

func (s *sender) prepareMessage(size int) *websocket.PreparedMessage {
	data := make([]byte, size)
	pm, err := websocket.NewPreparedMessage(websocket.BinaryMessage, data)
	if err != nil {
		pm = nil
	}
	return pm
}

func (s *sender) senderLoop(ctx context.Context) {
	size := SpecInitialBinaryMessageSize
	var pm *websocket.PreparedMessage
	var total int64
	for ctx.Err() == nil {
		var ri ReceiverInfo
		s.mutex.Lock()
		ri = s.ri
		s.ri = ReceiverInfo{}
		s.mutex.Unlock()
		if ri.ElapsedSeconds > 0.0 && ri.NumBytes > 0 {
			log.Printf("sender.senderLoop: %f %d", ri.ElapsedSeconds, ri.NumBytes)
			speed := float64(ri.NumBytes) / ri.ElapsedSeconds
			// Scale binary message to the desired size so that we reduce the
			// number of callbacks that we trigger on the receiver.
			const desiredSendsPerSecond = 16.0
			desiredSize := speed / desiredSendsPerSecond
			offset := uint(math.Ceil(math.Log2(desiredSize)))
			if offset < SpecInitialBinaryMessageSizeExponent {
				offset = SpecInitialBinaryMessageSizeExponent
			} else if offset > SpecMaxBinaryMessageSizeExponent {
				offset = SpecMaxBinaryMessageSizeExponent
			}
			integralSize := 1 << offset
			if integralSize > size {
				log.Printf("sender.senderLoop: scaling to %d", integralSize)
				size = integralSize
				pm = nil
			}
			// Decide whether we've created too much queue and, in such case, pause
			// the sender to give the queue some time t breathe.
			unsentByExcess := total - ri.NumBytes
			drainTime := float64(unsentByExcess) / speed
			const tooManySecondsOfQueue = 10.0
			if drainTime > tooManySecondsOfQueue {
				log.Printf("sender.senderLoop: drain time: %f", drainTime)
				time.Sleep(250 * time.Millisecond)
				continue
			}
		}
		if pm == nil {
			pm = s.prepareMessage(size)
			if pm == nil {
				log.Print("sender.senderLoop: cannot prepare message")
				return
			}
		}
		err := s.conn.WritePreparedMessage(pm)
		if err != nil {
			log.Printf("sender.senderLoop: conn.WritePreparedMessage: %s", err.Error())
			return
		}
		total += int64(size)
	}
}

func (s *sender) receiverLoop(ctx context.Context) {
	s.conn.SetReadLimit(SpecMaxTextMessageSize)
	for ctx.Err() == nil {
		mtype, mdata, err := s.conn.ReadMessage()
		if err != nil {
			log.Printf("sender.receiverLoop: conn.ReadMessage: %s", err.Error())
			return
		}
		if mtype != websocket.TextMessage {
			log.Print("sender.receiverLoop: non textual message")
			return
		}
		var ri ReceiverInfo
		err = json.Unmarshal(mdata, &ri)
		if err != nil {
			log.Printf("sender.receiverLoop: json.Unmarshal: %s", err.Error())
			return
		}
		if ri.ElapsedSeconds <= 0.0 || ri.NumBytes <= 0 {
			log.Print("sender.receiverLoop: message values out of range")
			return
		}
		//log.Printf("sender.receiverLoop: %f %d", ri.ElapsedSeconds, ri.NumBytes)
		s.mutex.Lock()
		s.ri = ri
		s.mutex.Unlock()
	}
}

func senderMain(ctx context.Context, conn *websocket.Conn) {
	defer conn.Close()
	const timeout = 10 * time.Second
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		log.Printf("senderMain: conn.SetReadDeadline: %s", err.Error())
		return
	}
	if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		log.Printf("senderMain: conn.SetWriteDeadline: %s", err.Error())
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s := &sender{conn: conn}
	go s.receiverLoop(ctx)
	s.senderLoop(ctx)
}
