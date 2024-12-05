#!/bin/bash

usage() {
  echo "Usage: $0 [-u Off-chain node URI] [-d Signed event HTTP body]" 1>&2
  exit 1
}

# Parse flags
BASE_URI=""
DATA=""

while getopts ":u:d:" opt; do
  case ${opt} in
    u)
      BASE_URI=$OPTARG
      ;;
    d)
      DATA=$OPTARG
      ;;
    :)
      usage
      ;;
    ?)
      echo "Invalid option: -${OPTARG}."
      exit 1
      ;;
  esac
done

# Check flags are not empty
if [ -z "$BASE_URI" ] || [ -z "$DATA" ]; then
  usage
fi

# Check Go is installed
if ! command -v go &> /dev/null; then
  echo "Go is not installed. Please install Go and try again."
  echo "Install Go: https://go.dev/doc/install"
  exit 1
fi

if ! command -v go-wrk &> /dev/null; then
  echo "go-wrk is not installed, commencing installation..."
  go install github.com/tsliwowicz/go-wrk@latest
fi

echo "Running benchmark..."

URL="$BASE_URI/event"
go-wrk -c 10 -d 10 -M POST -body="$DATA" "$URL"
