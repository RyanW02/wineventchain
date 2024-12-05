# Build stage
FROM golang:1.22-bookworm as builder

RUN apt-get update && apt-get upgrade -y && apt-get install -y ca-certificates curl make git

COPY / /tmp/compile
WORKDIR /tmp/compile

RUN cd offchain-interface && go build -o interface ./cmd/offchain-interface/main.go

# Production container
FROM golang:1.22-bookworm

RUN apt-get update && apt-get upgrade -y && apt-get install -y ca-certificates curl jq

COPY --from=builder /tmp/compile/offchain-interface/interface /srv/interface
RUN chmod +x /srv/interface

RUN useradd -m container
RUN mkdir -p /srv/data && chown container:container /srv/data

USER container
WORKDIR /srv

EXPOSE 8080
STOPSIGNAL SIGTERM
CMD ["/srv/interface"]

