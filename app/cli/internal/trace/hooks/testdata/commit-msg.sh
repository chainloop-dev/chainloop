#!/bin/sh
# chainloop-trace-managed
if command -v chainloop >/dev/null 2>&1; then
	chainloop trace hook git commit-msg "$@"
fi
exit 0
