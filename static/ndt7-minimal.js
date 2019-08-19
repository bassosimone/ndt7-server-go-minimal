/* jshint esversion: 6, asi: true, worker: true */
onmessage = function (ev) {
  'use strict'
  let url = new URL(ev.data.href)
  url.protocol = (url.protocol === 'https:') ? 'wss:' : 'ws:'
  const wsproto = 'net.measurementlab.ndt.v7'

  function download(callback) {
    url.pathname = '/ndt/v7/download'
    const sock = new WebSocket(url.toString(), wsproto)
    sock.onclose = function () {
      console.log(new Date().getTime())
      //callback()
    }
    sock.onopen = function () {
      console.log(new Date().getTime())
      const start = new Date().getTime()
      let previous = start
      let tot = 0
      sock.onmessage = function (ev) {
        tot += (ev.data instanceof Blob) ? ev.data.size : ev.data.length
        let now = new Date().getTime()
        const every = 250  // ms
        if (now - previous > every) {
          const message = {
            'ElapsedSeconds': (now - start) / 1000, // s
            'NumBytes': tot,
            'Origin': 'client',
            'SubTest': 'download',
          }
          sock.send(JSON.stringify(message))
          postMessage(message)
          previous = now
        }
      }
    }
  }

  download(function () {
    url.pathname = '/ndt/v7/upload'
    const sock = new WebSocket(url.toString(), wsproto)
    sock.onclose = function () {
      postMessage({
        'failure': null,
        'subTest': 'upload',
      })
    }

    function uploader(socket, data, start, previous, tot) {
      let now = new Date().getTime()
      const duration = 10000  // millisecond
      if (now - start > duration) {
        sock.close()
        return
      }
      // TODO(bassosimone): refine to ensure this works well across a wide
      // range of CPU speed/network speed/browser combinations
      const underbuffered = 7 * data.length
      while (sock.bufferedAmount < underbuffered) {
        sock.send(data)
        tot += data.length
      }
      const every = 250  // millisecond
      if (now - previous > every) {
        postMessage({
          'ElapsedSeconds': (now - start) / 1000,  // s
          'NumBytes': (tot - sock.bufferedAmount),
          'Origin': 'client',
          'SubTest': 'upload',
        })
        previous = now
      }
      // Message size adaptation algorithm. Estimate the current speed in
      // bytes per millisecond and, knowing that, how much time it would
      // require for us to drain the buffered data. Arrange so that we'll
      // sleep for half the time. Then, if we're sleeping for less than
      // a target amount of milliseconds, double the buffer. A key insight
      // is that we shall never reduce the buffer to avoid synchronising
      // the behaviour of TCP with the one of this buffer.
      const currentSpeed = (tot - sock.bufferedAmount) / (now - start)
      const nextSleep = (sock.bufferedAmount / currentSpeed) / 2
      const target = 50
      if (!isNaN(nextSleep) && nextSleep < target && data.length <= (1<<23)) {
        data = new Uint8Array(data.length << 1)
        // TODO(bassosimone): fill this message
      }
      setTimeout(function() {
        uploader(sock, data, start, previous, tot)
      }, nextSleep)
    }

    sock.onopen = function () {
      const data = new Uint8Array(1<<13)
      // TODO(bassosimone): fill this message
      sock.binarytype = 'arraybuffer'
      const start = new Date().getTime()
      const previous = start
      uploader(sock, data, start, previous, 0)
    }
  })
}
