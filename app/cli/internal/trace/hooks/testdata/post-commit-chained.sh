#!/bin/sh
# chainloop-trace-managed
if command -v chainloop >/dev/null 2>&1; then
	chainloop trace hook git post-commit
fi
[ -x "<hooks-dir>/post-commit.chainloop-backup" ] && exec "<hooks-dir>/post-commit.chainloop-backup" "$@"
exit 0
