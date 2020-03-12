/* jshint esversion: 6, asi: true */
// ndt7 is a simple ndt7 client.
const ndt7 = (function() {
  return {
    // run runs the specified test with the specified config. The config
    // object structure is as follows:
    //
    //     {
    //       baseURL: "",
    //       oncomplete: function() {},
    //       onmeasurement: function (measurement) {},
    //       onstarting: function() {},
    //       testName: "",
    //       userAcceptedDataPolicy: true
    //     }
    //
    // where baseURL is the mandatory URL; oncomplete, omeasurement, and
    // onstarting are optional handlers; testName is the mandatory test name
    // and must be one of "download", "upload"; userAcceptedDataPolicy is
    // a boolean that MUST be true for the measurement to start.
    //
    // The measurement is described by the ndt7 specification. See
    // https://github.com/m-lab/ndt-server/blob/master/spec/ndt7-protocol.md.
    run: function(config) {
      if (config === undefined || config.userAcceptedDataPolicy !== true) {
        throw "fatal: user must accept data policy first"
      }
      if (config.onstarting !== undefined) {
        config.onstarting()
      }
      let done = false
      let worker = new Worker("ndt7-" + config.testName + ".js")
      function finish() {
        if (!done) {
          done = true
          if (config.oncomplete !== undefined) {
            config.oncomplete()
          }
        }
      }
      worker.onmessage = function (ev) {
        if (ev.data === undefined) {
          finish()
          return
        }
        if (config.onmeasurement !== undefined) {
          config.onmeasurement(ev.data)
        }
      }
      // Kill the worker after the timeout. This force the browser to
      // close the WebSockets and prevent too-long tests.
      setTimeout(function () {
        worker.terminate()
        finish()
      }, 10000)
      worker.postMessage({
        href: config.baseURL,
      })
    }
  }
}())
