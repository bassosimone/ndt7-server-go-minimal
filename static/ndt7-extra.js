/* jshint esversion: 6, asi: true */
const ndt7extra = (function () {
  "use strict"

  function formatDate(date) {
    function pad(number) {
      if (number < 10) {
        return '0' + number;
      }
      return number;
    }
    return date.getUTCFullYear() +
      '-' + pad(date.getUTCMonth() + 1) +
      '-' + pad(date.getUTCDate()) +
      ' ' + pad(date.getUTCHours()) +
      ':' + pad(date.getUTCMinutes()) +
      ':' + pad(date.getUTCSeconds())
  }

  // newMeasurement returns an object that saves the result using
  // (a subset of) the OONI data format for ndt7.
  function newMeasurement() {
    const startTime = new Date()
    const startTimeString = formatDate(startTime)
    let measurement = {
      annotations: {
        real_data_format_version: "0.4.0",
      },
      data_format_version: "0.2.0",
      measurement_start_time: startTimeString,
      probe_asn: "AS0",
      probe_cc: "ZZ",
      probe_ip: "127.0.0.1",
      report_id: null,
      resolver_asn: "AS0",
      resolver_ip: "127.0.0.1",
      resolver_network_name: "",
      software_name: "bassosimone-ndt7js",
      software_version: "0.1.0-dev",
      test_keys: null,
      test_runtime: null,
      test_name: "ndt7",
      test_start_time: startTimeString,
      test_version: "0.1.0"
    }
    let tk = {
      download: [],
      failure: null,
      summary: {
        avg_rtt: null,
        download: null,
        mss: null,
        max_rtt: null,
        min_rtt: null,
        ping: null,
        retransmit_rate: null,
        upload: null,
      },
      upload: []
    }
    function computeSpeed(ai) {
      const millisec = ai.ElapsedTime / 1e03
      const bits = ai.NumBytes * 8
      return bits / millisec /* kbit/s */
    }
    return {
      update: function (ev) {
        if (ev.Test === "download") {
          tk.download.push(ev)
          if (ev.TCPInfo !== undefined && ev.TCPInfo !== null) {
            const rtt = ev.TCPInfo.RTT / 1e03 /* us => ms */
            tk.summary.avg_rtt = rtt
            tk.summary.mss = ev.TCPInfo.AdvMSS
            if (tk.summary.max_rtt === null || tk.summary.max_rtt < rtt) {
              tk.summary.max_rtt = rtt
            }
            tk.summary.min_rtt = ev.TCPInfo.MinRTT / 1e03 /* us => ms */
            tk.summary.ping = tk.summary.min_rtt
            if (ev.TCPInfo.BytesSent > 0) {
              tk.summary.retransmit_rate = ev.TCPInfo.BytesRetrans / ev.TCPInfo.BytesSent
            }
          }
          if (ev.AppInfo !== undefined && ev.AppInfo !== null) {
            tk.summary.download = computeSpeed(ev.AppInfo)
          }
        } else if (ev.Test === "upload") {
          tk.upload.push(ev)
          if (ev.AppInfo !== undefined && ev.AppInfo !== null) {
            tk.summary.upload = computeSpeed(ev.AppInfo)
          }
        } else {
          throw "update: unexpected ev.Test value"
        }
      },
      end: function () {
        const endTime = new Date()
        const testRuntime = (endTime.getTime() - startTime.getTime()) / 1e03
        const rv = Object.assign({}, measurement)
        rv.test_runtime = testRuntime
        rv.test_keys = Object.assign({}, tk)
        return rv
      },
    }
  }

  return {
    newMeasurement: newMeasurement,
  }
})()
