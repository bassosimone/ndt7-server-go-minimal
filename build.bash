#!/bin/bash
set -ex
export GO111MODULE=on
go build -o ndt7-server-bin -v .
