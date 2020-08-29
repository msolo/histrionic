#!/bin/bash

# NOTE(msolo) If this gets any longer, rewrite in python.

has_error=0
function fatal {
    has_error=1
    echo "$*" >&2
    exit 1
}

set -e

# Always run from git root.
cd $(git rev-parse --show-toplevel)

histrionic=./cmd/histrionic/histrionic
history_file_in="./test/history-in"

archive_file_out="./test-out/archive-out.hjs"
history_file_out="./test-out/history-out"
search_file_out="./test-out/search-out.hjs"
session_file_out="./test-out/session-out.hjs"

archive_file_golden="./test/archive-golden.hjs"
history_file_golden="./test/history-golden"
search_file_golden="./test/search-golden.hjs"
session_file_golden="./test/session-golden.hjs"

rm -rf ./test-out
mkdir -p -m 755 "./test-out"

function on_exit() {
  if [[ $has_error == 0 ]]; then
    rm -rf ./test-out
  else
    echo "error occurred, leaving debug data in ./test-out" >&2
  fi
}

function on_error() {
  rc=$1
  lineno=$2
  has_error=1
  echo "error $rc occurred on line $lineno" >&2
}

trap 'on_exit' EXIT
trap 'on_error $? $LINENO' ERR

$histrionic import -bash-histfile $history_file_in -hostname test-hostname -o $archive_file_out

# Create timestamp newer than the existing entries.
timestamp=1598728455

echo "echo session command 1" | $histrionic append -exit-code 0 -session test-session -hostname test-hostname -timestamp $timestamp -o $session_file_out
echo "echo session command 2" | $histrionic append -exit-code 1 -session test-session -hostname test-hostname -timestamp $timestamp -o $session_file_out

$histrionic merge -o $archive_file_out $archive_file_out $session_file_out

$histrionic dump -history-fmt -o $history_file_out -coalesce -prune $archive_file_out

$histrionic dump -coalesce -prune -o $search_file_out $session_file_out $archive_file_out

diff -u $archive_file_out $archive_file_golden || fatal "archive check failed $archive_file_out"

diff -u $session_file_out $session_file_golden || fatal "archive check failed $session_file_out"

diff -u $history_file_out $history_file_golden || fatal "archive check failed $history_file_out"

diff -u $search_file_out $search_file_golden || fatal "archive check failed $search_file_out"

echo "OK - Great success."
exit 0
