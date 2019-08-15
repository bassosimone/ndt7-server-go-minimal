package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	bulkMessageSize = 1 << 13
	defaultTimeout  = 7 * time.Second
)

type measurer struct {
	ticker *time.Ticker
	begin  time.Time
}

func newMeasurer() *measurer {
	return &measurer{
		ticker: time.NewTicker(250 * time.Millisecond),
		begin:  time.Now(),
	}
}

func (m *measurer) stop() {
	m.ticker.Stop()
}

func (m *measurer) maybePrint(total int, subtest string) {
	select {
	case <-m.ticker.C:
		fmt.Printf(`{"ElapsedSeconds":%f,"SubTest":"%s","NumBytes":%d}`+"\n",
			time.Now().Sub(m.begin).Seconds(), subtest, total)
	default:
		// nothing
	}
}

func downloadupload(
	ctx context.Context, timeout time.Duration, subtest string,
	preparedMessage *websocket.PreparedMessage, conn *websocket.Conn,
) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn.SetReadLimit(1 << 24)
	meas := newMeasurer()
	defer meas.stop()
	var total int
	for ctx.Err() == nil {
		if preparedMessage == nil {
			if err := conn.SetReadDeadline(time.Now().Add(defaultTimeout)); err != nil {
				return
			}
			_, mdata, err := conn.ReadMessage()
			if err != nil {
				return
			}
			total += len(mdata)
		} else {
			if err := conn.SetWriteDeadline(time.Now().Add(defaultTimeout)); err != nil {
				return
			}
			if err := conn.WritePreparedMessage(preparedMessage); err != nil {
				return
			}
			total += bulkMessageSize
		}
		meas.maybePrint(total, subtest)
	}
}

func upgrade(w http.ResponseWriter, r *http.Request) *websocket.Conn {
	const proto = "net.measurementlab.ndt.v7"
	if r.Header.Get("Sec-WebSocket-Protocol") != proto {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	h := http.Header{}
	h.Add("Sec-WebSocket-Protocol", proto)
	var u websocket.Upgrader
	conn, err := u.Upgrade(w, r, h)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	return conn
}

var endpoint = flag.String("endpoint", ":8080", "Endpoint to listen to")

func main() {
	http.HandleFunc("/ndt/v7/download", func(w http.ResponseWriter, r *http.Request) {
		conn := upgrade(w, r)
		if conn == nil {
			return
		}
		defer conn.Close()
		data := make([]byte, bulkMessageSize)
		if _, err := rand.Read(data); err != nil {
			return
		}
		pm, err := websocket.NewPreparedMessage(websocket.BinaryMessage, data)
		if err != nil {
			return
		}
		downloadupload(r.Context(), 10*time.Second, "download", pm, conn)
	})
	http.HandleFunc("/ndt/v7/upload", func(w http.ResponseWriter, r *http.Request) {
		if conn := upgrade(w, r); conn != nil {
			defer conn.Close()
			downloadupload(r.Context(), 15*time.Second, "upload", nil, conn)
		}
	})
	http.ListenAndServe(*endpoint, nil)
}
