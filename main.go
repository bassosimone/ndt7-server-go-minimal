package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	minMessageSize       = 1 << 10
	maxScaledMessageSize = 1 << 20
	maxMessageSize       = 1 << 24
	maxRuntime           = 10 * time.Second
	measureInterval      = 250 * time.Millisecond
	fractionForScaling   = 16
)

func emitAppInfo(start time.Time, total int64, testname string) {
	fmt.Printf(`{"AppInfo":{"NumBytes":%d,"ElapsedTime":%d},"Test":"%s"}`+"\n",
		total, time.Since(start)/time.Microsecond, testname)
}

func uploadTest(ctx context.Context, conn *websocket.Conn) error {
	var total int64
	start := time.Now()
	if err := conn.SetReadDeadline(start.Add(maxRuntime)); err != nil {
		return err
	}
	conn.SetReadLimit(maxMessageSize)
	ticker := time.NewTicker(measureInterval)
	defer ticker.Stop()
	for ctx.Err() == nil {
		kind, reader, err := conn.NextReader()
		if err != nil {
			return err
		}
		if kind == websocket.TextMessage {
			data, err := ioutil.ReadAll(reader)
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", string(data))
		}
		n, err := io.Copy(ioutil.Discard, reader)
		if err != nil {
			return err
		}
		total += int64(n)
		select {
		case <-ticker.C:
			emitAppInfo(start, total, "download")
		default:
			// NOTHING
		}
	}
	return nil
}

func newMessage(n int) (*websocket.PreparedMessage, error) {
	return websocket.NewPreparedMessage(websocket.BinaryMessage, make([]byte, n))
}

func downloadTest(ctx context.Context, conn *websocket.Conn) error {
	var total int64
	start := time.Now()
	if err := conn.SetWriteDeadline(time.Now().Add(maxRuntime)); err != nil {
		return err
	}
	size := minMessageSize
	message, err := newMessage(size)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(measureInterval)
	defer ticker.Stop()
	for ctx.Err() == nil {
		if err := conn.WritePreparedMessage(message); err != nil {
			return err
		}
		total += int64(size)
		select {
		case <-ticker.C:
			emitAppInfo(start, total, "upload")
		default:
			// NOTHING
		}
		if int64(size) >= maxScaledMessageSize || int64(size) >= (total/fractionForScaling) {
			continue
		}
		size <<= 1
		if message, err = newMessage(size); err != nil {
			return err
		}
	}
	return nil
}

func upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	const proto = "net.measurementlab.ndt.v7"
	if r.Header.Get("Sec-WebSocket-Protocol") != proto {
		w.WriteHeader(http.StatusBadRequest)
		return nil, errors.New("Missing Sec-WebSocket-Protocol header")
	}
	h := http.Header{}
	h.Add("Sec-WebSocket-Protocol", proto)
	u := websocket.Upgrader{
		ReadBufferSize:  maxMessageSize,
		WriteBufferSize: maxMessageSize,
	}
	return u.Upgrade(w, r, h)
}

var (
	flagCert     = flag.String("cert", "cert.pem", "TLS certificate to use")
	flagEndpoint = flag.String("endpoint", ":443", "Endpoint to listen to")
	flagKey      = flag.String("key", "key.pem", "TLS private key to use")
)

func main() {
	flag.Parse()
	http.HandleFunc("/ndt/v7/download", func(w http.ResponseWriter, r *http.Request) {
		if conn, err := upgrade(w, r); err == nil {
			downloadTest(r.Context(), conn)
		}
	})
	http.HandleFunc("/ndt/v7/upload", func(w http.ResponseWriter, r *http.Request) {
		if conn, err := upgrade(w, r); err == nil {
			uploadTest(r.Context(), conn)
		}
	})
	http.Handle("/", http.FileServer(http.Dir("static")))
	log.Fatal(http.ListenAndServeTLS(*flagEndpoint, *flagCert, *flagKey, nil))
}
