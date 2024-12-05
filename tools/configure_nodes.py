#!/usr/bin/env python3

import json
import os
import getopt
import subprocess
import sys
import tomlkit
from datetime import datetime

USAGE = "Usage: python configure_nodes.py [--use-mongodb=true|false] <path_to_docker_volumes>"

if len(sys.argv) < 2:
    print(USAGE)
    sys.exit(1)

options, arguments = getopt.getopt(
    sys.argv[1:],
    'm:',
    ["use-mongodb="]
)

if len(arguments) < 1:
    print(USAGE)
    sys.exit(1)

use_mongo_db = False

for option, value in options:
    if option in ('-m', '--use-mongodb'):
        use_mongo_db = value.lower() == 'true'

path = os.getcwd()
if not path.endswith(os.sep):
    path += os.sep

path += ' '.join(arguments)
node_dirs = os.listdir(path)

node_ids = {}
for dir in node_dirs:
    result = subprocess.run(['cometbft', 'show_node_id', '--home=' + path + os.sep + dir + os.sep + 'cometbft-data'], stdout=subprocess.PIPE)
    node_ids[dir] = result.stdout.decode('utf-8').strip()

pub_keys = {}
for dir in node_dirs:
    validator_key_path = path + os.sep + dir + os.sep + 'cometbft-data' + os.sep + 'config' + os.sep + 'priv_validator_key.json'
    with open(validator_key_path, 'r') as f:
        data = json.load(f)
        pub_keys[dir] = data['pub_key']

genesis_time = datetime.utcnow().isoformat('T') + 'Z'

for dir in node_dirs:
    config_path = path + os.sep + dir + os.sep + 'cometbft-data' + os.sep + 'config'
    if not os.path.exists(config_path):
        print("Config directory not found for node: " + dir)
        sys.exit(1)

    # Configure nodes to connect to each other
    config = {}
    with open(config_path + os.sep + 'config.toml', 'r') as f:
        data = f.read()
        config = tomlkit.parse(data)

    config['rpc']['laddr'] = 'tcp://0.0.0.0:26657'
    config['p2p']['addr_book_strict'] = False

    if use_mongo_db:
        config['db_backend'] = 'mongodb'
        config['db_options'] = {
            'connection_string': 'mongodb://root:root@{}-mongodb:27017'.format(dir),
            'database': 'blockstore'
        }

    peers = {name: node_id for name, node_id in node_ids.items() if name != dir}
    config['p2p']['persistent_peers'] = ','.join(["{}@{}:{}".format(node_id, name, 26656) for name, node_id in peers.items()])

    with open(config_path + os.sep + 'config.toml', 'w') as f:
        f.write(tomlkit.dumps(config))

    # Set up validator config
    genesis_data = {}
    with open(config_path + os.sep + 'genesis.json', 'r') as f:
        genesis_data = json.load(f)

    genesis_data['genesis_time'] = genesis_time
    genesis_data['chain_id'] = 'wineventchain'

    # Configure the self node as a validator
    genesis_data['validators'][0]['power'] = '10'
    genesis_data['validators'][0]['name'] = dir

    # Configure the other nodes as validators
    for name, node_id in node_ids.items():
        if name != dir:
            genesis_data['validators'].append({
                'pub_key': pub_keys[name],
                'power': '10',
                'name': name
            })

    with open(config_path + os.sep + 'genesis.json', 'w') as f:
        f.write(json.dumps(genesis_data, indent=2))
