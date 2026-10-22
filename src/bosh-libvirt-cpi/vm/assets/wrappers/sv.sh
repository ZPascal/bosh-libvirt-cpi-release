#!/bin/sh
# Stub: intercept all sv verbs so the monit stub is never killed.
case "$1" in
  start)       echo "ok: run: $2: (pid 0) 1s"; exit 0 ;;
  stop)        echo "ok: down: $2: 0s";        exit 0 ;;
  kill|force-stop) echo "ok: down: $2: 0s";   exit 0 ;;
  status)      echo "run: $2: (pid 0) 1s";    exit 0 ;;
esac
exec /usr/bin/sv "$@"
