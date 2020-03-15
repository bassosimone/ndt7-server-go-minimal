/* jshint esversion: 6, asi: true */
const ndt7core = (function () {
  "use strict"

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
  function locate(config) {
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
        config.callback("https://" + doc.fqdn + "/")
      })
  }

  // startWorker starts a test in a worker. The config object structure is:
  //
  //     {
  //       baseURL: "",
  //       ontestcomplete: function() {},
  //       ontestmeasurement: function (measurement) {},
  //       onteststarting: function() {},
  //       testName: "",
  //       userAcceptedDataPolicy: true
  //     }
  //
  // where
  //
  // - `baseURL` (`string`) is the mandatory http/https URL of the server (use
  //   the `locate` function to get the URL of the server);
  //
  // - `ontestcomplete` (`function(testSpec)`) is the optional callback called
  //   when done (see below for the testSpec structure);
  //
  // - `ontestmeasurement` (`function(measurement)`) is the optional callback
  //   called when a new measurement object is emitted (see below);
  //
  // - `onteststarting` is like `ontestcomplete` but called at startup;
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
  function startWorker(config) {
    if (config === undefined || config.userAcceptedDataPolicy !== true) {
      throw "fatal: user must accept data policy first"
    }
    if (config.testName !== "download" && config.testName !== "upload") {
      throw "fatal: testName is neither download nor upload"
    }
    if (config.baseURL === undefined || config.baseURL === "") {
      throw "fatal: baseURL not provided"
    }
    if (config.onteststarting !== undefined) {
      config.onteststarting({
        "Origin": "client",
        "Test": config.testName,
      })
    }
    const start = new Date().getTime()
    let done = false
    let worker = new Worker("ndt7-" + config.testName + ".js")
    function finish(error) {
      if (!done) {
        done = true
        const stop = new Date().getTime()
        if (config.ontestcomplete !== undefined) {
          config.ontestcomplete({
            "Origin": "client",
            "Test": config.testName,
            "WorkerInfo": {
              "ElapsedTime": (stop - start) * 1000, // us
              "Error": error,
            },
          })
        }
      }
    }
    worker.onerror = function (ev) {
      finish(ev.message || "Terminated with exception")
    }
    worker.onmessage = function (ev) {
      if (ev.data === null) {
        finish(null)
        return
      }
      if (config.ontestmeasurement !== undefined) {
        config.ontestmeasurement(ev.data)
      }
    }
    // Kill the worker after the timeout. This forces the browser to
    // close the WebSockets and prevents too-long tests.
    const killAfter = 10000 // ms
    setTimeout(function () {
      worker.terminate()
      finish("Terminated with timeout")
    }, killAfter)
    worker.postMessage({
      baseURL: config.baseURL,
    })
  }

  function startTest(config, url, testName, callback) {
    startWorker({
      baseURL: url,
      onteststarting: config.onteststarting,
      ontestmeasurement: config.ontestmeasurement,
      ontestcomplete: function (ev) {
        if (config.ontestcomplete !== undefined) {
          config.ontestcomplete(ev)
        }
        callback()
      },
      testName: testName,
      userAcceptedDataPolicy: config.userAcceptedDataPolicy,
    })
  }

  // start starts the ndt7 test suite. The config object structure is:
  //
  //     {
  //       baseURL: "",
  //       oncomplete: function() {},
  //       onstarting: function() {},
  //       ontestcomplete: function (testSpec) {},
  //       ontestmeasurement: function (measurement) {},
  //       onteststarting: function (testSpec) {},
  //       userAcceptedDataPolicy: true
  //     }
  //
  // where
  //
  // - `baseURL` (`string`) is the optional http/https URL of the server (we
  //   will locate a suitable server if this is missing);
  //
  // - `oncomplete` (`function(testSpec)`) is the optional callback called
  //   when the whole test suite has finished;
  //
  // - `onstarting` is like `oncomplete` but called at startup;
  //
  // - `onserverurl` (`function(string)`) is called when we have located
  //   the server URL, or immediately if you provided a baseURL;
  //
  // - `ontestcomplete` is exactly like the `ontestcomplete` field passed
  //   to the `startWorker` function (see above);
  //
  // - `ontestmeasurement` is exactly like the `ontestmeasurement` field passed
  //   to the `startWorker` function (see above);
  //
  // - `onteststarting` is exactly like the `onteststarting` field passed
  //   to the `startWorker` function (see above);
  //
  // - `userAcceptedDataPolicy` MUST be present and set to true otherwise
  //   this function will immediately throw an exception.
  function start(config) {
    if (config.onstarting !== undefined) {
      config.onstarting()
    }
    locate({
      callback: function (url) {
        config.onserverurl(url)
        startTest(config, url, "download", function () {
          startTest(config, url, "upload", config.oncomplete)
        })
      },
      mockedResult: config.baseURL,
      userAcceptedDataPolicy: config.userAcceptedDataPolicy,
    })
  }

  return {
    locate: locate,
    startWorker: startWorker,
    start: start,
  }
})()