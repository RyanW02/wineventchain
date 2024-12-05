#!/usr/bin/env python3

import csv
import json
import sys

if len(sys.argv) < 2:
    print("Usage: python events_to_json.py <path_to_events_csv> <output_file>")
    sys.exit(1)

with open(sys.argv[1], 'r') as f:
    reader = csv.reader(f)
    next(reader)  # Skip header
    events = list(reader)

    export = {int(event[2]): event[3] for event in events}

    with open(sys.argv[2], 'w') as out:
        json.dump(export, out, indent=4)

