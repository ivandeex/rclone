#!/bin/bash
#set -x

remote="$1"
mountpoint="$2"
shift 2

wait=yes
bglog=no
foreground=no
rclone="/usr/bin/rclone"
args=""

export PATH=/bin:/usr/bin
export RCLONE_CONFIG="/etc/rclone/rclone.conf"
export RCLONE_VERBOSE=0

# Process -o parameters
while getopts :o: opts; do
  if [ "$opts" != "o" ]; then
    echo "invalid option: -${OPTARG}"
    continue
  fi

  params=${OPTARG//,/ }
  for param in $params; do
    case "$param" in
      # generic mount options
      rw|ro|dev|nodev|suid|nosuid|exec|noexec|auto|noauto|user)
        continue ;;
      # systemd options
      _netdev|nofail|x-systemd.*)
        continue ;;
      # wrapper options
      proxy=*)
        export http_proxy=${param#*=}
        export https_proxy=${param#*=} ;;
      config=*)
        export RCLONE_CONFIG=${param#*=} ;;
      verbose=*)
        export RCLONE_VERBOSE=${param#*=} ;;
      nowait)
        wait=no ;;
      foreground)
        foreground=yes ;;
      bglog)
        bglog=yes ;;
      # fuse / rclone options
      allow_other|allow_root|uid=*|gid=*)
        args="$args --${param//_/-}" ;;
      # rclone options
      *)
        args="$args --$param" ;;
    esac
  done
done

if [ $bglog = yes ]; then
  stamp=$(date '+%y%m%d-%H%M%S')
  pid=$$
  where=$(basename "$mountpoint")
  logfile=/tmp/rclone-${stamp}-${pid}-${where}.log
  touch "$logfile"
  chmod 666 "$logfile"
  # activate verbose background logging
  export RCLONE_VERBOSE=3
  export RCLONE_LOG_FORMAT=date,time,microseconds
  export RCLONE_LOG_FILE=$logfile
  # deactivate systemd log flavor in rclone
  unset INVOCATION_ID
fi

# exec rclone (shellcheck note: args must stay unquoted)
if [ $foreground = yes ]; then
  # shellcheck disable=SC2086
  exec "$rclone" mount $args "$remote" "$mountpoint"
else
  # NOTE: --daemon hangs under systemd automount, using `&`
  # shellcheck disable=SC2086
  "$rclone" mount $args "$remote" "$mountpoint" </dev/null >&/dev/null &
  while [ $wait = yes ] && [ "$(grep -c " ${mountpoint%/} fuse.rclone " /proc/mounts)" = 0 ]; do
    sleep 0.5
  done
fi
