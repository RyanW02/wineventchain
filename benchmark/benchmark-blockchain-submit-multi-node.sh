#!/bin/bash

usage() {
  echo "Usage: $0 [-t seconds]" 1>&2
  exit 1
}

# Parse flags
SECONDS=""

while getopts ":t:" opt; do
  case ${opt} in
    t)
      SECONDS=$OPTARG
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

# Check Go is installed
if ! command -v go &> /dev/null; then
  echo "Go is not installed. Please install Go and try again."
  echo "Install Go: https://go.dev/doc/install"
  exit 1
fi

go run cmd/blockchain/main.go -c 1 -T $SECONDS -r 125 -s 125 \
    --broadcast-tx-method sync \
    --endpoints ws://localhost:26647/websocket,ws://localhost:26649/websocket,ws://localhost:26651/websocket,ws://localhost:26653/websocket
