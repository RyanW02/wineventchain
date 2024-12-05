# Build stage
FROM golang:1.22-bookworm as builder

RUN apt-get update && apt-get upgrade -y && apt-get install -y ca-certificates curl make git

COPY / /tmp/compile
WORKDIR /tmp/compile

RUN cd wineventchain && go build -o node ./cmd/app/main.go

# Production container
FROM golang:1.22-bookworm

RUN apt-get update && apt-get upgrade -y && apt-get install -y ca-certificates curl jq

COPY --from=builder /tmp/compile/wineventchain/node /node/node
RUN chmod +x /node/node

RUN useradd -m container

RUN mkdir -p /node/state
RUN chown -R container /node/state

USER container
WORKDIR /node

EXPOSE 26657
STOPSIGNAL SIGTERM
CMD ["/node/node", "--tendermint_config", "/node/data/config/config.toml"]

