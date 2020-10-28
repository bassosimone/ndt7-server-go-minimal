/* jshint esversion: 6, asi: true, worker: true */
// WebWorker that runs the ndt7 roundtrip test
onmessage = function (ev) {
  "use strict"
  let url = new URL(ev.data.baseURL)
  url.protocol = (url.protocol === "https:") ? "wss:" : "ws:"
  url.pathname = "/ndt/v7/roundtrip"
  const sock = new WebSocket(url.toString(), "net.measurementlab.ndt.v7")
  sock.onclose = function () {
    postMessage(null)
  }
  sock.onopen = function () {
    const start = new Date().getTime()
    sock.onmessage = function (ev) {
      if ((ev.data instanceof Blob)) {
        // TODO(bassosimone): is this the correct action here?
        throw "unexpected message type"
      }
      const m = JSON.parse(ev.data)
      const now = new Date().getTime()
      sock.send(JSON.stringify({
        STE: m.ST,
        STD: (now - start) * 1000 - m.ST,
        RT: (now - start) * 1000,
      }))
      postMessage({
        "AppInfo": {
          "ElapsedTime": (now - start) * 1000,  // us
          "SRTT": m.SRTT,
        },
        "Origin": "server",
        "Test": "roundtrip",
      })
    }
  }
}
