/* jshint esversion: 6, asi: true */
// ndt7 is a simple ndt7 client.
const ndt7 = (function() {
  return {
    // run runs the specified test with the specified base URL and calls
    // handler's callbacks to notify the caller of ndt7 events.
    run: function(baseURL, testName, handler) {
      if (handler !== undefined && handler.onstarting !== undefined) {
        handler.onstarting({Origin: 'client', Test: testName})
      }
      let done = false
      let worker = new Worker('ndt7-' + testName + '.js')
      function finish() {
        if (!done) {
          done = true
          if (handler !== undefined && handler.oncomplete !== undefined) {
            handler.oncomplete({Origin: 'client', Test: testName})
          }
        }
      }
      worker.onmessage = function (ev) {
        if (ev.data === null) {
          finish()
          return
        }
        if (handler !== undefined && handler.onmeasurement !== undefined) {
          handler.onmeasurement(ev.data)
        }
      }
      // Kill the worker after the timeout. This force the browser to
      // close the WebSockets and prevent too-long tests.
      setTimeout(function () {
        worker.terminate()
        finish()
      }, 10000)
      worker.postMessage({
        href: baseURL,
      })
    }
  }
}())
