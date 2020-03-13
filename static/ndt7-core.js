/* jshint esversion: 6, asi: true */
// ndt7 is a simple ndt7 client.
const ndt7 = (function() {
  function startWithBaseURL(config) {
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
    // Kill the worker after the timeout. This forces the browser to
    // close the WebSockets and prevent too-long tests.
    setTimeout(function () {
      worker.terminate()
      finish()
    }, 10000)
    worker.postMessage({
      href: config.baseURL,
    })
  }

  function locate(config) {
    if (config.url === undefined || config.url === "") {
      config.url = "https://locate.measurementlab.net/ndt7"
    }
    fetch(config.url)
      .then(function (response) {
        return response.json()
      })
      .then(function (doc) {
        config.callback(doc.fqdn)
      })
  }

  return {
    // start starts the specified test with the specified config. The config
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
    // where
    //
    // - `baseURL` (`string`) is the optional http/https URL of the server;
    //
    // - `oncomplete` (`function()`) is the optional callback called when done;
    //
    // - `onlocate` (`function({fqdn: ""})`) is the optional callback called when
    //   the locate service has discovered a valid server;
    //
    // - `onmeasurement` (`function(measurement)`) is the optional callback
    //   called when a new measurement object is emitted (see below);
    //
    // - `onstarting` is like `oncomplete` but called at startup;
    //
    // - `testName` (`string`) is one of "download", "upload";
    //
    // - `userAcceptedDataPolicy` MUST be present and set to true.
    //
    // If `baseURL` is missing, we will locate a nearby server using mlab-ns.
    //
    // The measurement object is described by the ndt7 specification. See
    // https://github.com/m-lab/ndt-server/blob/master/spec/ndt7-protocol.md.
    start: function start(config) {
      if (config === undefined || config.userAcceptedDataPolicy !== true) {
        throw "fatal: user must accept data policy first"
      }
      if (config.baseURL !== undefined && config.baseURL !== "") {
        startWithBaseURL(config)
        return
      }
      locate({
        callback: function (fqdn) {
          if (config.onlocate !== undefined) {
            config.onlocate({"fqdn": fqdn.split(".")[0]})
          }
          config.baseURL = "https://" + fqdn
          startWithBaseURL(config)
        },
      })
    }
  }
}())
