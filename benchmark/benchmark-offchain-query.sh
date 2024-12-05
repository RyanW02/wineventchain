#!/bin/bash

usage() {
  echo "Usage: $0 [-e event_id] [-u Off-Chain Node Server URI] [-t seconds]" 1>&2
  exit 1
}

# Parse flags
EVENT_ID=""
OFFCHAIN_URI=""
TIME="10"

while getopts ":e:u:t:" opt; do
  case ${opt} in
    e)
      EVENT_ID=$OPTARG
      ;;
    u)
      OFFCHAIN_URI=$OPTARG
      ;;
    t)
      TIME=$OPTARG
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
if [ -z "$EVENT_ID" ] || [ -z "$OFFCHAIN_URI" ]; then
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

URL="$OFFCHAIN_URI/event/$EVENT_ID"
go-wrk -c 10 -d "$TIME" "$URL"
