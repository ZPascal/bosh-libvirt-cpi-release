#!/bin/sh
# Replace 'su - user -c cmd' with setpriv to avoid PAM issues in containers.
# setpriv is in util-linux and switches uid/gid without PAM authentication.
_user=root
while [ $# -gt 0 ]; do
  case "$1" in
    -c) shift
      if [ "$_user" = root ]; then
        exec /bin/sh -c "$@"
      fi
      _uid=$(id -u "$_user" 2>/dev/null || echo 1000)
      _gid=$(id -g "$_user" 2>/dev/null || echo 1000)
      exec setpriv --reuid="$_uid" --regid="$_gid" --clear-groups -- /bin/sh -c "$@"
      ;;
    -*) shift ;;
    *)  _user="$1"; shift ;;
  esac
done
