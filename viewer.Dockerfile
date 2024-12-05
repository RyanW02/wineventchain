# SvelteKit build stage
FROM node:21-bullseye AS spa-builder

ENV DEFAULT_API_URL=http://localhost:4000

RUN apt-get update && apt-get upgrade -y && apt-get install -y ca-certificates curl make git

COPY / /tmp/build
WORKDIR /tmp/build

RUN cd viewer/public && npm ci && npm run build

# Server stage
FROM golang:1.22-bookworm as server-builder

RUN apt-get update && apt-get upgrade -y && apt-get install -y ca-certificates curl make git

COPY / /tmp/compile
WORKDIR /tmp/compile

RUN cd viewer && go build -o server ./cmd/viewer/main.go

# Production container
FROM golang:1.22-bookworm

RUN apt-get update && apt-get upgrade -y && apt-get install -y ca-certificates curl jq

COPY --from=server-builder /tmp/compile/viewer/server /srv/server
RUN chmod +x /srv/server

COPY --from=spa-builder /tmp/build/viewer/public/build /srv/public

RUN useradd -m container

USER container
WORKDIR /srv

EXPOSE 4000
STOPSIGNAL SIGTERM
CMD ["/srv/server"]
