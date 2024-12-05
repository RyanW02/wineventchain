# Configuration
The application can be configured using environment variables (preferred in containerised environments), or a JSON
configuration file. The application will look for a file named `config.json` in the current working directory, and use
it if it exists. If the file does not exist, the application will use environment variables.

## JSON Configuration
There is a `config.json.example` file in the current directory that can be used as a template for creating a
configuration file.

## Environment Variables
The available environment variables are as follows:
- `PRODUCTION` - Set to `true` to enable production mode. Affects various aspects such as log formatting and HTTP debug
endpoints.
- `PRETTY_LOGS` - Set to `true` to enable pretty log formatting. Set to `false` to use JSON log formatting.
- `LOG_LEVEL` - The log level to use. Possible values are, `debug`, `info`, `warn` and `error`.
- `SERVER_ADDRESS` - The address the server will bind to.
- `VIEWER_SERVER_ENABLED` - Whether to enable the API server functionality to serve data to the frontend event viewer
website.
- `VIEWER_SERVER_ADDRESS` - The address the viewer server will bind to if enabled: this must be different to the main
server address to separate endpoints for security reasons.
- `VIEWER_SERVER_JWT_ALGORITHM` - The algorithm to use for signing JWTs. The only supported algorithm is `HS256`.
- `VIEWER_SERVER_JWT_SECRET` - The secret to use for signing JWTs. Must be a string of at least 32 characters for HS256.
- `VIEWER_SERVER_CHALLENGE_LIFETIME` - The amount of time (e.g. `5m`) that clients have to respond to an authentication
challenge before it is invalidated.
- `VIEWER_SERVER_SEARCH_PAGE_LIMIT` - The maximum number of events to return in a single event search result.
- `BLOCKCHAIN_NODE_ADDRESSES` - A comma separated list of CometBFT node addresses.
- `BLOCKCHAIN_MINIMUM_NODES` - The minimum number of nodes required to start the application. Nodes may go offline and
come back online later, but the application will not start until this number of nodes are online.
- `MONGODB_URI` - The connection string for the MongoDB database.
- `MONGODB_DATABASE_NAME` - The name of the MongoDB database.
- `STATE_PATH` - The path to the directory that should be used for the local state database.
- `TRANSPORT_NODE_NAME` - The *unique* identifier for the node used in the gossip transport layer. You should ideally 
use small node names to reduce the header size of packets.
- `TRANSPORT_BIND_ADDRESS` - The address the transport layer will bind to.
- `TRANSPORT_BIND_PORT` - The port the transport layer will bind to.
- `TRANSPORT_NETWORK_TYPE` - The network architecture that the cluster is deployed in. Possible values are `local`,
`lan` and `wan`. Affects various tuning parameters in the gossip transport layer.
- `TRANSPORT_RETRANSMIT_MULTIPLIER` - The multiplier to use to calculate how many times a frame should be retransmitted:
the formula is `retransmitMultiplier * log10(numberOfNodes)`.
- `TRANSPORT_USE_GOSSIP` - Set to `true` to use gossip-based multicast for event propagation. Due to the nature of
gossip protocols, this may lead to events not being propagated to all nodes in the network in some cases - however,
nodes will notice missing events and make backfill requests. If set to `false`, the node that received the event will
send it directly to all other nodes in the network using unicast: this may be impractical in large networks. The
aforementioned backfill behaviour will still occur in this mode, meaning that if a node fails to distribute an event
to all peers, the peers missing the event will backfill it from other nodes.
- `TRANSPORT_PEERS` - A comma-separated list of peers to connect to, in the format of `address:port`.
- `TRANSPORT_USE_ENCRYPTION` - Whether to use encryption in the transport layer. If set to `true`, `TRANSPORT_SHARED_KEY`
must also be set.
- `TRANSPORT_SHARED_KEY` - The shared key to use for encryption in the transport layer. Must be a 16-byte or 32-byte, to
enable AES-128 or AES-256 encryption respectively.
- `BACKFILL_TRY_UNICAST_FIRST` - Whether to try unicast first when backfilling state. If set to `true`, the node will
request state from a single peer first, and if that fails, it will request state from all peers. If set to `false`, the
node will start by requesting state over multicast.
- `BACKFILL_BLOCK_POLL_INTERVAL` - The interval at which the node will poll the local state database for missing blocks,
- `BACKFILL_BLOCK_FETCH_CHUNK_SIZE` - The number of blocks to request at a time from the blockchain when backfilling
state.
- `BACKFILL_EVENT_POLL_INTERVAL` - The interval at which the node will poll the local state database for missing events.
- `BACKFILL_EVENT_FETCH_CHUNK_SIZE` - The number of events to request at a time from neighbouring nodes when backfilling
state.
- `BACKFILL_EVENT_NEW_EVENT_IGNORE_THRESHOLD` - The duration (e.g. `5m`) to ignore missing events for once they have
been discovered before requesting them over the network. This is to prevent the node from requesting new events that
are yet to be submitted to the off-chain network and propagated.
- `BACKFILL_EVENT_RETRY_INTERVAL` - The interval at which the node will retry fetching missing events from neighbouring
nodes.
- `BACKFILL_EVENT_MAX_RETRIES` - The maximum number of times the node will retry fetching the same missing event from
neighbouring nodes before removing it from the missing events list and assuming the event will never be seen.
- `BACKFILL_MULTICAST_BACKOFF` - The duration (e.g. `5s`) to backoff for after each multicast request when backfilling 
state. This is to prevent the node from flooding the network with requests when it is behind.
- `EVENT_RETENTION_RUN_AT_STARTUP` - Whether to run the event retention policy eviction process at startup, rather than
waiting an `EVENT_RETENTION_SCAN_INTERVAL` cycle.
- `EVENT_RETENTION_SCAN_INTERVAL` - The duration (e.g. `1h`) between each event retention policy eviction process. This
is a potentially intensive background process, so it is recommended to choose a suitably long interval.
- `EVENT_RETENTION_SCAN_TIMEOUT` - The maximum duration (e.g. `30m`) an event retention policy eviction process is
allowed to run for.
