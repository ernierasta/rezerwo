#!/bin/sh
# Runs second rezerwo instance for browser testing (e.g. by Claude in Chrome)
# next to the dev one on :3002. Open http://localhost:3003/admin/devlogin to be
# logged in as admin REZERWO_DEV_LOGIN without password.
#
# usage (from repo root): REZERWO_DEV_LOGIN=admin@email tools/test-server.sh [addr]

set -e

if [ -z "$REZERWO_DEV_LOGIN" ]; then
	echo "set REZERWO_DEV_LOGIN to email of admin user" >&2
	exit 1
fi

ADDR=${1:-127.0.0.1:3003}
mkdir -p tmp
CGO_ENABLED=0 go build -o tmp/rezerwo-test .
REZERWO_ADDR=$ADDR exec tmp/rezerwo-test
