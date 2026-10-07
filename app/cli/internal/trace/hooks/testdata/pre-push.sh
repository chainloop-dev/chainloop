#!/bin/sh
# chainloop-trace-managed
if command -v chainloop >/dev/null 2>&1; then
	chainloop trace hook git pre-push || exit $?
fi
exit 0
