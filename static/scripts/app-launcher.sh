#!/bin/bash
RESOURCES="$(cd "$(dirname "$0")/../Resources" && pwd)"
BINARY="$RESOURCES/setup"

if [ -t 1 ]; then
  exec "$BINARY"
fi

osascript -e "
tell application \"Terminal\"
  activate
  do script quoted form of \"$BINARY\"
end tell
" &
exit 0
