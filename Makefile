.DEFAULT_GOAL := fresh

fresh: build reset run-testnet-recreate

run-testnet:
	docker compose up

run-testnet-recreate:
	docker compose up --force-recreate

build: build-blockchain build-offchain build-viewer build-tooling

build-blockchain:
	docker build -f blockchain.Dockerfile -t wineventchain .

build-offchain:
	docker build -f offchain.Dockerfile -t offchain-interface .

build-viewer:
	docker build -f viewer.Dockerfile -t viewer .

build-tooling:
	cd tools && \
		python3 -m venv . && \
		. bin/activate && \
		pip install -r requirements.txt

reset: reset-with-goleveldb
reset-with-goleveldb: initialise configure-with-goleveldb
reset-with-mongodb: initialise configure-with-mongodb

initialise:
	docker compose down -v
	sudo rm -rf docker-volumes
	# Create the directories for the nodes
	mkdir -p docker-volumes/blockchain-01/cometbft-data
	mkdir -p docker-volumes/blockchain-02/cometbft-data
	mkdir -p docker-volumes/blockchain-03/cometbft-data
	mkdir -p docker-volumes/blockchain-04/cometbft-data
	# Initialise the data directories with the CometBFT config
	cometbft init --home docker-volumes/blockchain-01/cometbft-data
	cometbft init --home docker-volumes/blockchain-02/cometbft-data
	cometbft init --home docker-volumes/blockchain-03/cometbft-data
	cometbft init --home docker-volumes/blockchain-04/cometbft-data

configure-with-goleveldb:
	# Run script to update the config files
	. tools/bin/activate && \
		python3 tools/configure_nodes.py docker-volumes/

configure-with-mongodb:
	# Run script to update the config files
	. tools/bin/activate && \
		python3 tools/configure_nodes.py --use-mongodb=true docker-volumes/