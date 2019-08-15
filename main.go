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
			total += *bulkMessageSize
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

func main() {
	flag.Parse()
	http.HandleFunc("/ndt/v7/download", func(w http.ResponseWriter, r *http.Request) {
		conn := upgrade(w, r)
		if conn == nil {
			return
		}
		defer conn.Close()
		data := make([]byte, *bulkMessageSize)
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
	if *cert != "" && *key != "" {
		http.ListenAndServeTLS(*endpoint, *cert, *key, nil)
	} else {
		http.ListenAndServe(*endpoint, nil)
	}
}
