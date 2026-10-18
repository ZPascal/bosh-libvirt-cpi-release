#!/bin/sh
case "$*" in
  *systemd-resolved*) exit 0 ;;
  *monit*) exit 0 ;;
  *) exit 1 ;;
esac
