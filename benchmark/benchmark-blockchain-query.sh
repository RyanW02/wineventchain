#!/bin/bash

usage() {
  echo "Usage: $0 [-e event_id] [-u Tendermint RPC server URI] [-p Generate proofs]" 1>&2
  exit 1
}

# Parse flags
EVENT_ID=""
TENDERMINT_RPC_URI=""
PROOF="false"

while getopts ":e:u:p" opt; do
  case ${opt} in
    e)
      EVENT_ID=$OPTARG
      ;;
    u)
      TENDERMINT_RPC_URI=$OPTARG
      ;;
    p)
      PROOF="true"
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
if [ -z "$EVENT_ID" ] || [ -z "$TENDERMINT_RPC_URI" ]; then
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

if [ "$PROOF" = "true" ]; then
  echo "Benchmarking blockchain query with proof generation..."
else
  echo "Benchmarking blockchain query without proof generation..."
fi

URL="$TENDERMINT_RPC_URI/abci_query?path=\"/event-by-id/$EVENT_ID\"&data=\"{\\\"app\\\":\\\"events\\\"}\"&height=0&prove=$PROOF"
go-wrk -c 10 -d 10 "$URL"
