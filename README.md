# WinEventChain

## Introduction

This repository contains an implementation for the storage, retrieval and analysis of Windows event logs using a
blockchain and off-chain storage system.

Key features include:

- The storage of core event data on the [CometBFT](https://cometbft.com/) blockchain.
- The storage of additional event data (the <EventData> section of Windows event logs, which contain potentially
  sensitive information) on a distributed system of off-chain nodes.
- An event collection agent, written in Rust, which subscribes to Windows event log streams, and sends the event data to
  blockchain and off-chain nodes, with the ability to intermittently retry submission on node failure.
- The automatic fan-out of event data between off-chain nodes
  using [Hashicorp Memberlist](https://github.com/hashicorp/memberlist), a
  [SWIM](https://www.cs.cornell.edu/projects/Quicksilver/public_pdfs/SWIM.pdf) based gossip multicast protocol.
- The automatic removal of data stored on the offchain nodes, based on customisable retention policies (see the
  [retention policies documentation file](./retention-policies.md) for more information).
- The ability to search and view in real-time the event data using a web interface.

## Deployment

Deployment of the system is easy, using Docker compose and Makefiles.

### Pre-requisites

Please ensure that you have the following software installed:

- [Docker](https://docker.com), to containerise the software.
- [Docker Compose](https://docs.docker.com/compose/), for easy deployment of the many services.
- [Make](https://www.gnu.org/software/make/), to run the deployment commands.
- [Python](https://www.python.org/), to run the configuration tooling.
- [CometBFT](https://docs.cometbft.com/v0.38/guides/install), to initialise the blockchain files.
- [Rust](https://www.rust-lang.org/), to build the event collection agent.
- [Go](https://go.dev/), to build the CLI tool for interacting with the blockchain applications.
- A Windows machine or VM, to build the event collection agent.

### Deployment Steps

The `docker-compose.yml` file is used to deploy a 4 node testnet, running on the local machine. In a production
environment, there should be one CometBFT node and one off-chain node deployed per machine.

1. Run `make fresh` to build the Docker images, generate the necessary configuration files, and start the services.
   Note that this command will **delete the existing Docker volumes for the services**, so only run this command on the
   first start, or to reset the system.
2. Use `make run-testnet` to start the services in the future without rebuilding Docker images or resetting the
   configurations.
3. Enter the `chain-client` directory and use `make run` or `make build` to run the CLI tool. A `config.json` file will
be generated: specify the blockchain node address and run the application again. Then, use the identity app to seed
the blockchain with an admin user (principal) and private key.
4. The generated admin principal can then be used to create new principals and private keys for each machine that the
event collection agent will run on, to identify the machine submitting events.
5. Follow the steps in the [Agent Deployment](#agent-deployment) section to deploy the agent on the client machines.
6. Use the web interface ([http://localhost:3000](http://localhost:3000)) to view the event logs.

By default, the Docker compose configuration will expose the following services on the following ports:

| **Port**    | **Service**                                                                                             |
|-------------|---------------------------------------------------------------------------------------------------------|
| 3000        | The web app for viewing event logs (`viewer`).                                                          |
| 4000,4001   | The offchain node servers for retrieving event data in the viewer.                                      |
| 8080,8081   | The offchain node servers for ingesting events from agents - use these in the agent `config.toml` file. |
| 26646,26647 | The RPC ports for the 1ˢᵗ CometBFT blockchain node.                                                     |
| 26648,26649 | The RPC ports for the 2ⁿᵈ CometBFT blockchain node.                                                     |
| 26650,26651 | The RPC ports for the 3ʳᵈ CometBFT blockchain node.                                                     |
| 26652,26653 | The RPC ports for the 4ᵗʰ CometBFT blockchain node.                                                     |

It is worth reviewing the `Makefile`. There are many tasks for configuring the system in different manners, such as
`make reset-with-mongodb`, to configure the system to use MongoDB as the backend for Tendermint's blockstore data.

### Agent Deployment

1. Enter the `agent` directory.
2. On a *Windows machine*, run the command `make build` if available. Otherwise, manually run the following commands:
    ```
    rustup target add x86_64-pc-windows-gnu
    cargo build --release && mv ./target/x86_64-pc-windows-gnu/release/agent.exe .
    ```
3. An `agent.exe` binary will be built
4. Copy the `agent.exe` binary to the client machines that will have event logs collected from. Deploy the agent as
   a background service, *running as administrator*, as the Win32 APIs used to collect events require administrator
   privileges to access.
5. On first run, the agent will generate a `config.toml` file in the current directory, and then exit. Edit the
   `config.toml` file to include the node addresses, agent private key (generated using the `chain-client` CLI tool)
   and adjust other parameters as necessary.
6. Run the agent again, and it will begin collecting and submitting event logs to the blockchain and off-chain nodes.

## Directory Structure
This repository serves as a monorepo, containing the source code for all services:

```
├── agent # Rust source code for the event collection agent
├── benchmark # Scripts used to benchmark the system
├── chain-client # CLI tool to interact with the blockchain applications
├── cometbft # A fork of CometBFT, allowing storage of blockchain data in MongoDB
├── cometbft-db # A fork of CometBFT-DB, allowing storage of blockchain data in MongoDB
├── common # Common code shared between multiple services, such as payload definitions
├── offchain-interface # Source code for the off-chain node servers
├── tools # Configuration generation Python tooling
├── viewer # The web application for viewing event logs
│   ├── cmd # The HTTP file server for the web application
│   └── public # The frontend SvelteKit application 
└── wineventchain # The blockchain applications, running on-chain
```

---

## Why is there only 1 commit?
I overwrite the commit history for my privacy :)
