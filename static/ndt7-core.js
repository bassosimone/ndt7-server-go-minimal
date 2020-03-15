/* jshint esversion: 6, asi: true */

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
// - `callback` (`function(url)`) is the mandatory callback called on
//   success with the stringified URL of the located server as argument;
//
// - `mockedResult` (`string`) optionally allows you to skip the real
//   location lookup and immediately callback with the provided string;
//
// - `url` (`string`) is the optional locate service URL;
//
// - `userAcceptedDataPolicy` MUST be present and set to true otherwise
//   this function will throw an exception.
export function locate(config) {
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
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      return response.json()
    })
    .then(function (doc) {
      config.callback(`https://${doc.fqdn}`)
    })
}

// startWorker starts a test in a worker. The config object structure is:
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
// - `oncomplete` (`function(testSpec)`) is the optional callback called
//   when done (see below for the testSpec structure);
//
// - `onmeasurement` (`function(measurement)`) is the optional callback
//   called when a new measurement object is emitted (see below);
//
// - `onstarting` is like `oncomplete` but called at startup;
//
// - `testName` (`string`) must be one of "download", "upload";
//
// - `userAcceptedDataPolicy` MUST be present and set to true otherwise
//   this function will immediately throw an exception.
//
// The measurement object is described by the ndt7 specification. See
// https://github.com/m-lab/ndt-server/blob/master/spec/ndt7-protocol.md.
//
// The testSpec structure is like:
//
//     {
//       "Origin": "client",
//       "Test": ""
//     }
//
// where Origin is always "client" and Test is "download" or "upload".
export function startWorker(config) {
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
    config.onstarting({
      "Origin": "client",
      "Test": config.testName,
    })
  }
  let done = false
  let worker = new Worker(`ndt7-${config.testName}.js`)
  function finish() {
    if (!done) {
      done = true
      if (config.oncomplete !== undefined) {
        config.oncomplete({
          "Origin": "client",
          "Test": config.testName,
        })
      }
    }
  }
  worker.onmessage = function (ev) {
    if (ev.data === null) {
      finish()
      return
    }
    if (config.onmeasurement !== undefined) {
      config.onmeasurement(ev.data)
    }
  }
  // Kill the worker after the timeout. This forces the browser to
  // close the WebSockets and prevents too-long tests.
  const killAfter = 10000 // ms
  setTimeout(function () {
    worker.terminate()
    finish()
  }, killAfter)
  worker.postMessage({
    baseURL: config.baseURL,
  })
}
