/* jshint esversion: 6, asi: true, worker: true */
onmessage = function (ev) {
  'use strict'
  let url = new URL(ev.data.href)
  url.protocol = (url.protocol === 'https:') ? 'wss:' : 'ws:'
  const wsproto = 'net.measurementlab.ndt.v7'
  url.pathname = '/ndt/v7/upload'
  const sock = new WebSocket(url.toString(), wsproto)
  sock.onclose = function () {
    postMessage(null)
  }
  function uploader(socket, data, start, previous, total) {
    let now = new Date().getTime()
    const duration = 10000  // millisecond
    if (now - start > duration) {
      sock.close()
      return
    }
    if (data.length < (1<<24) && data.length < (total - sock.bufferedAmount)/16) {
      data = new Uint8Array(data.length << 1) // TODO(bassosimone): fill this message
    }
    const underbuffered = 7 * data.length
    while (sock.bufferedAmount < underbuffered) {
      sock.send(data)
      total += data.length
    }
    const every = 250  // millisecond
    if (now - previous > every) {
      postMessage({
        'AppInfo': {
          'ElapsedTime': (now - start) * 1000,  // us
          'NumBytes': (total - sock.bufferedAmount),
        },
        'Test': 'upload',
      })
      previous = now
    }
    const drainSpeed = (total - sock.bufferedAmount) / (now - start)
    const nextSleep = (sock.bufferedAmount / drainSpeed) / 2
    setTimeout(function() {
      uploader(sock, data, start, previous, total)
    }, nextSleep)
  }
  sock.onopen = function () {
    const data = new Uint8Array(1<<13) // TODO(bassosimone): fill this message
    sock.binarytype = 'arraybuffer'
    const start = new Date().getTime()
    uploader(sock, data, start, start, 0)
  }
}
