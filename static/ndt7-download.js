/* jshint esversion: 6, asi: true, worker: true */
onmessage = function (ev) {
  'use strict'
  let url = new URL(ev.data.href)
  url.protocol = (url.protocol === 'https:') ? 'wss:' : 'ws:'
  url.pathname = '/ndt/v7/download'
  const sock = new WebSocket(url.toString(), 'net.measurementlab.ndt.v7')
  sock.onclose = function () {
    postMessage(null)
  }
  sock.onopen = function () {
    const start = new Date().getTime()
    let previous = start
    let total = 0
    sock.onmessage = function (ev) {
      total += (ev.data instanceof Blob) ? ev.data.size : ev.data.length
      let now = new Date().getTime()
      const every = 250  // ms
      if (now - previous > every) {
        postMessage({
          'AppInfo': {
            'ElapsedTime': (now - start) * 1000,  // us
            'NumBytes': total,
          },
          'Test': 'download',
        })
        previous = now
      }
    }
  }
}