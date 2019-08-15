# Minimal ndt7 server written in Go

This repository contains a minimal ndt7 server written in Go. It lacks many
functionality implemented by [better servers](
https://github.com/m-lab/ndt-server). It's used as a benchmark to make
sure the production client still works with minimal servers.

To build, make sure you have Go >= 1.11 installed and then run

```bash
./build.bash
```

To serve clear-text incoming requests on port `8080` run

```bash
./ndt7-server-bin | ./ndt7-server-aux
```

Omit `./ndt7-server-aux` if you don't want to pretty-print the speed. Run

```bash
./ndt7-server-bin -h
```

to get more help. See also the documentation at the top of [main.go](main.go).
