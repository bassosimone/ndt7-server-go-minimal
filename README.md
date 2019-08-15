# Minimal ndt7 server written in Go

This repository contains a minimal ndt7 server written in Go. It lacks many
functionality implemented by [better servers](
https://github.com/m-lab/ndt-server). It's used as a benchmark to make
sure the production client still works with minimal servers.

To build, make sure you have Go >= 1.11 installed and then run

```bash
./build.bash
```

To serve TLS incoming requests on port `443` run

```
./gencerts.bash
sudo ./enable-bbr.bash
sudo ./ndt7-server-bin | ./ndt7-server-aux
```

The server includes a minimal web client that you can use for testing.

Omit `./ndt7-server-aux` if you don't want to pretty-print the speed. Run

```bash
./ndt7-server-bin -h
```

to get an online help showing command line flags.
