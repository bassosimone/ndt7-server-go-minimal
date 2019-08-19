// ndt7-server-bin is a minimal ndt7 command line server.
//
// Usage:
//
//    ndt7-server-bin [-bulk-message-size <size>] [-cert <cert>]
//                    [-endpoint <epnt>] [-key <key>]
//
// The `-bulk-message-size <size>` flag allows you to set the size of the
// binary WebSocket messages used to create network load.
//
// The `-cert <cert>` and `-key <key>` flags allow to set the certificate
// and key used by TLS. If either of these is not set, we will listen
// for plain-text WebSocket, otherwise we'll do secure WebSocket.
//
// The `-endpoint <epnt>` flag allows you to control the TCP endpoint
// where we will listen for ndt7 clients requests.
//
// Additionally, passing any unrecognized flag, such as `-help`, will
// cause ndt7-client-bin to print a brief help message.
//
// While running, this client emits JSON events separated by newlines on
// the standard output. These events tell you the number of bytes downloaded or
// uploaded after a certain amount of seconds have elapsed. For example:
//
//     {"SubTest":"download","ElapsedSeconds":1.0,"NumBytes":12345}
//
// This program never terminates. Use ^C to interrupt it.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultTimeout = 7 * time.Second
)

var (
	bulkMessageSize = flag.Int("bulk-message-size", 1<<13, "WebSocket bulk messages size")
	cert            = flag.String("cert", "", "TLS certificate to use")
	endpoint        = flag.String("endpoint", ":8080", "Endpoint to listen to")
	key             = flag.String("key", "", "TLS private key to use")
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

func (m *measurer) maybePrint(total int, subtest string) (elapsed float64) {
	select {
	case now := <-m.ticker.C:
		elapsed = now.Sub(m.begin).Seconds()
		fmt.Printf(`{"ElapsedSeconds":%f,"SubTest":"%s","NumBytes":%d}`+"\n",
			elapsed, subtest, total)
	default:
		// nothing
	}
	return
}

func newPreparedMessage() *websocket.PreparedMessage {
	data := make([]byte, *bulkMessageSize)
	if _, err := rand.Read(data); err != nil {
		return nil
	}
	pm, err := websocket.NewPreparedMessage(websocket.BinaryMessage, data)
	if err != nil {
		return nil
	}
	return pm
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
			meas.maybePrint(total, subtest)
		} else {
			if err := conn.SetWriteDeadline(time.Now().Add(defaultTimeout)); err != nil {
				return
			}
			if err := conn.WritePreparedMessage(preparedMessage); err != nil {
				return
			}
			total += *bulkMessageSize
			elapsed := meas.maybePrint(total, subtest)
			// If a measurement interval has elapsed, compute the current send
			// buffer filling speed. Estimate the amount of data we should fill
			// in every reasonably small interval. Then scale the message we
			// are sending if it's greater than before and we don't exceed the
			// limit. We never decrease the message size to avoid syncing the
			// TCP behaviour with the buffer scaling behaviour.
			if elapsed > 0.0 {
				currentSpeed := float64(total) / elapsed
				amount := int(currentSpeed * 0.05) * 2
				if amount > 0 && amount > *bulkMessageSize {
					if amount > (1<<24) {
						amount = 1<<24
					}
					*bulkMessageSize = amount
					preparedMessage = newPreparedMessage()
					if preparedMessage == nil {
						return
					}
				}
			}
		}
	}
}

type Message struct {
	ElapsedSeconds float64
	NumBytes       int64
	Origin         string
	SubTest        string
}

func download(
	ctx context.Context, timeout time.Duration, subtest string,
	preparedMessage *websocket.PreparedMessage, conn *websocket.Conn,
) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	mchan := make(chan Message)
	// Receive speed samples from the client
	go func() {
		conn.SetReadLimit(1 << 24)
		defer close(mchan)
		if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			return
		}
		for ctx.Err() == nil {
			mtype, mdata, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if mtype != websocket.TextMessage {
				return
			}
			var message Message
			err = json.Unmarshal(mdata, &message)
			if err != nil {
				return
			}
			mchan <- message
		}
	}()
	// Send data to the client
	if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return
	}
	start := time.Now()
	var queued int64
	for ctx.Err() == nil {
		select {
		case message := <-mchan:
			if message.ElapsedSeconds > 0.0 {
				speed := float64(message.NumBytes) / message.ElapsedSeconds
				realElapsed := time.Now().Sub(start).Seconds()
				fmt.Printf("realElapsed: %f\n", realElapsed)
				// Possibly stop early if we've buffered too much already
				if realElapsed >= message.ElapsedSeconds {
					maybeSent := int64(speed * (realElapsed - message.ElapsedSeconds))
					notSentEstimate := queued - (message.NumBytes + maybeSent)
					if notSentEstimate > 0 {
						eta := (float64(notSentEstimate) / speed) + realElapsed
						fmt.Printf("ETA: %f\n", eta)
						if eta >= 15.0 {
							return
						}
					}
				}
				// Adapt the bulk message size to reduce receiver's overhead
				optimalSize := int64(speed * 0.1)
				if int(optimalSize) > *bulkMessageSize {
				} else if int(optimalSize) < *bulkMessageSize {
				}
				if int(optimalSize) > *bulkMessageSize && *bulkMessageSize < (1 << 23) {
					fmt.Printf("scaling: %d => %d\n", *bulkMessageSize, *bulkMessageSize * 2)
					*bulkMessageSize *= 2
					preparedMessage = newPreparedMessage()
					if preparedMessage == nil {
						return
					}
				}
			}
			continue // Process consecutive messages
		default:
			// NOTHING
		}
		if err := conn.WritePreparedMessage(preparedMessage); err != nil {
			return
		}
		queued += int64(*bulkMessageSize)
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

func main() {
	flag.Parse()
	http.HandleFunc("/ndt/v7/download", func(w http.ResponseWriter, r *http.Request) {
		conn := UpgraderUpgrade(w, r)
		if conn != nil {
			SenderMain(r.Context(), conn)
		}
	})
	http.HandleFunc("/ndt/v7/upload", func(w http.ResponseWriter, r *http.Request) {
		if conn := upgrade(w, r); conn != nil {
			defer conn.Close()
			downloadupload(r.Context(), 15*time.Second, "upload", nil, conn)
		}
	})
	http.Handle("/", http.FileServer(http.Dir("static")))
	if *cert != "" && *key != "" {
		http.ListenAndServeTLS(*endpoint, *cert, *key, nil)
	} else {
		http.ListenAndServe(*endpoint, nil)
	}
}
