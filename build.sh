#!/usr/bin/env bash

go build -o library/param_pusher_darwin
GOOS=linux go build -o library/param_pusher_linux