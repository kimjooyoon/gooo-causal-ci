#!/usr/bin/env bash
set -euo pipefail

exec go -C fixtures/subject test -count=1 -json .
