#!/usr/bin/env bash
# What this fork keeps on top of yetone/magpie, checked after a merge of
# upstream: the sync workflow runs it before it pushes, the release before
# it builds, and so can you after resolving a merge by hand.
#
#   - the macOS ids are the fork's own, never upstream's (new upstream code
#     and tests bring them back)
#   - no daily PostHog count: internal/stats is a stub and no key is linked
#     (new upstream code calls what the stub dropped)
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# as the terminal binaries are built, and so it runs on a bare Linux runner
export CGO_ENABLED=0

# git grep, but an error fails rather than reading as no match
found() {
	local rc=0
	git grep -n "$@" || rc=$?
	[ "$rc" -le 1 ] || exit "$rc"
	[ "$rc" = 0 ]
}

fail=0
if found 'com\.yetone\.magpie' -- '*.go' 'build/' 'internal/' ':!internal/update/relaunch_fallback_test.go'; then
	echo "fork-check: upstream's bundle ids are back, use com.zhsama.magpie" >&2
	fail=1
fi
if found 'phc_' -- '*.go' 'internal/'; then
	echo "fork-check: a PostHog key is in the app" >&2
	fail=1
fi
[ "$fail" = 0 ]

go build -tags nogui ./...
go vet ./internal/stats/ ./internal/daemon/ ./internal/autostart/
go test -count=1 ./internal/stats/ ./internal/daemon/
echo "fork-check: ok"
