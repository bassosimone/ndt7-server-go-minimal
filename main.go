package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
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

	roundTripInterval       = 100 * time.Millisecond
	roundTripMaxMessageSize = 1 << 17
	roundTripRuntime        = 3 * time.Second
)

type roundTripRequest struct {
	RTTVar float64       // RTT variance (μs)
	SRTT   float64       // smoothed RTT (μs)
	ST     time.Duration // sender time (μs)
}

type roundTripReply struct {
	STE time.Duration // sender time echo (μs)
	STD time.Duration // sender time difference (μs)
	RT  time.Duration // receiver time (μs)
}

func roundTripRecvReply(conn *websocket.Conn) (*roundTripReply, error) {
	kind, reader, err := conn.NextReader()
	if err != nil {
		return nil, err
	}
	if kind != websocket.TextMessage {
		return nil, errors.New("unexpected message type")
	}
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	var reply roundTripReply
	if err := json.Unmarshal(data, &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}

type roundTripStats struct {
	SRTT     float64
	RTTVar   float64
	nsamples int64
}

func (rts roundTripStats) String(elapsed time.Duration, prefix string) string {
	return fmt.Sprintf(
		`{"AppInfo":{"Smoothed%s":%f,"%sVar":%f,"ElapsedTime":%d},"Test":"%s"}`,
		prefix, rts.SRTT, prefix, rts.RTTVar, elapsed, "roundtrip")
}

func (rts *roundTripStats) update(sample float64) {
	// See https://tools.ietf.org/html/rfc6298
	const (
		alpha = 0.125
		beta  = 0.25
	)
	rts.nsamples++
	if rts.nsamples == 1 {
		rts.SRTT = sample
		rts.RTTVar = sample / 2
		return
	}
	rts.RTTVar = (1-beta)*rts.RTTVar + beta*math.Abs(rts.RTTVar-sample)
	rts.SRTT = (1-alpha)*rts.SRTT + alpha*sample
}

func roundTripTest(ctx context.Context, conn *websocket.Conn) error {
	start := time.Now()
	if err := conn.SetReadDeadline(start.Add(roundTripRuntime)); err != nil {
		return err
	}
	if err := conn.SetWriteDeadline(start.Add(roundTripRuntime)); err != nil {
		return err
	}
	conn.SetReadLimit(roundTripMaxMessageSize)
	ticker := time.NewTicker(roundTripInterval)
	defer ticker.Stop()
	var (
		sendstats roundTripStats
		recvstats roundTripStats
		rttstats  roundTripStats
	)
	for ctx.Err() == nil {
		request := roundTripRequest{
			RTTVar: rttstats.RTTVar,
			SRTT:   rttstats.SRTT,
			ST:     time.Since(start) / time.Microsecond,
		}
		if err := conn.WriteJSON(request); err != nil {
			return err
		}
		reply, err := roundTripRecvReply(conn)
		if err != nil {
			return err
		}
		// TODO(bassosimone): here we could potentially check whether
		// the client is cheating by asserting that request.ST == reply.STE
		// and we could otherwise stop the experiment
		elapsed := time.Since(start) / time.Microsecond
		sendstats.update(float64(reply.STD))
		fmt.Printf("%s\n\n", sendstats.String(elapsed, "SNDT"))
		// TODO(bassosimone): like we do in the client here we could
		// improve elapsed by computing it in roundTripRecvReply.
		RTD := float64(elapsed - reply.RT)
		recvstats.update(RTD)
		fmt.Printf("%s\n\n", recvstats.String(elapsed, "RCVT"))
		rttsample := float64(elapsed - reply.STE)
		rttstats.update(rttsample)
		fmt.Printf("%s\n\n", rttstats.String(elapsed, "RTT"))
		// Implementation note: the main purpose of waiting here is to
		// avoid flooding the client if RTT < 100 ms.
		<-ticker.C
	}
	return nil
}

func emitAppInfo(start time.Time, total int64, testname string) {
	fmt.Printf(`{"AppInfo":{"NumBytes":%d,"ElapsedTime":%d},"Test":"%s"}`+"\n\n",
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
	flagCert              = flag.String("cert", "cert.pem", "TLS certificate to use")
	flagEndpointCleartext = flag.String("endpoint-cleartext", ":80", "Cleartext endpoint to listen to")
	flagEndpointTLS       = flag.String("endpoint-tls", ":443", "TLS endpoint to listen to")
	flagKey               = flag.String("key", "key.pem", "TLS private key to use")
)

func main() {
	flag.Parse()
	http.HandleFunc("/ndt/v7/roundtrip", func(w http.ResponseWriter, r *http.Request) {
		if conn, err := upgrade(w, r); err == nil {
			roundTripTest(r.Context(), conn)
		}
	})
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
	go func() {
		log.Fatal(http.ListenAndServeTLS(*flagEndpointTLS, *flagCert, *flagKey, nil))
	}()
	go func() {
		log.Fatal(http.ListenAndServe(*flagEndpointCleartext, nil))
	}()
	<-context.Background().Done()
}
