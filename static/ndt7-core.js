/* jshint esversion: 6, asi: true */
// ndt7 is a simple ndt7 client.
const ndt7 = (function() {
  return {
    // locate locates the closest server. The config object structure is:
    //
    //     {
    //       callback: function(url) {},
    //       mockedResult: "",
    //       url: "",
    //       userAcceptedDataPolicy: true
    //     }
    //
    // where:
    //
    // - `callback` (`function(url)`) is the callback called on success
    //   with the URL of the located server;
    //
    // - `mockedResult` (`string`) allows you to skip the real location and
    //   immediately return the provided result to the caller;
    //
    // - `url` (`string`) is the optional locate service URL;
    //
    // - `userAcceptedDataPolicy` MUST be present and set to true.
    locate: function (config) {
      if (config === undefined || config.userAcceptedDataPolicy !== true) {
        throw "fatal: user must accept data policy first"
      }
      if (config.mockedResult !== undefined && config.mockedResult !== "") {
        config.callback(config.mockedResult)
        return
      }
      if (config.url === undefined || config.url === "") {
        config.url = "https://locate.measurementlab.net/ndt7"
      }
      fetch(config.url)
        .then(function (response) {
          return response.json()
        })
        .then(function (doc) {
          config.callback(`https://${doc.fqdn}`)
        })
    },

    // start starts a test. The config object structure is:
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
    // - `baseURL` (`string`) is the mandatory http/https URL of the server (use
    //   the `locate` function to get the URL of the server);
    //
    // - `oncomplete` (`function()`) is the optional callback called when done;
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
    // The measurement object is described by the ndt7 specification. See
    // https://github.com/m-lab/ndt-server/blob/master/spec/ndt7-protocol.md.
    start: function(config) {
      if (config === undefined || config.userAcceptedDataPolicy !== true) {
        throw "fatal: user must accept data policy first"
      }
      if (config.testName !== "download" && config.testName !== "upload") {
        throw "fatal: testName is neither download nor upload"
      }
      if (config.baseURL === undefined || config.baseURL === "") {
        throw "fatal: baseURL not provided"
      }
      if (config.onstarting !== undefined) {
        config.onstarting()
      }
      let done = false
      let worker = new Worker(`ndt7-${config.testName}.js`)
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
        baseURL: config.baseURL,
      })
    },
  }
}())
