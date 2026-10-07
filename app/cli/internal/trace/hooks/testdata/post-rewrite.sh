#!/bin/sh
# chainloop-trace-managed
if command -v chainloop >/dev/null 2>&1; then
	chainloop trace hook git post-rewrite
fi
exit 0
