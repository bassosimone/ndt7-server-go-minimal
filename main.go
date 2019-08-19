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
	"flag"
	"net/http"
)

var (
	cert            = flag.String("cert", "", "TLS certificate to use")
	endpoint        = flag.String("endpoint", ":8080", "Endpoint to listen to")
	key             = flag.String("key", "", "TLS private key to use")
)

func main() {
	flag.Parse()
	http.HandleFunc("/ndt/v7/download", func(w http.ResponseWriter, r *http.Request) {
		conn := UpgraderUpgrade(w, r)
		if conn != nil {
			senderMain(r.Context(), conn)
		}
	})
	http.HandleFunc("/ndt/v7/upload", func(w http.ResponseWriter, r *http.Request) {
		if conn := UpgraderUpgrade(w, r); conn != nil {
			receiverMain(r.Context(), conn)
		}
	})
	http.Handle("/", http.FileServer(http.Dir("static")))
	if *cert != "" && *key != "" {
		http.ListenAndServeTLS(*endpoint, *cert, *key, nil)
	} else {
		http.ListenAndServe(*endpoint, nil)
	}
}
