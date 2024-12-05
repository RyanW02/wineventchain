# Retention Policies
## Introduction
This document contains example off-chain data retention policy files. Retention policies can only be deployed once under
the current model, to prevent malicious actors from deploying a rule that would allow them to prematurely remove and
censor event data.

## Deploying Policies
To deploy a policy file, use the `chain-client`. Enter the `chain-client` directory and run the client using the following
command:
```bash
go run cmd/client.main.go
```

Then, select the following options:
1. 📲 Interact with node
2. 🧲 Event Retention Policy
3. 📦 Deploy Policy File
4. Enter the file path of the YAML policy file to deploy

## Policy Schema
Any events that do not match a policy will be dropped. Therefore, it is important to have at least one filter defined.

The root of the policy file is a map with a single key, `filters`, which contains a list of filter items.
```yaml
filters:
  - ...
  - ...
```

Each filter has a human-readable `label`, an optional `match` block and a `policy` block.

The `match` block is used to filter events based on their properties. You may specify zero, one or more of the following
properties - the conditions are combined with a logical AND:
```yaml
match:
  channel: "security" # The name of the event channel, e.g. "security", "system", etc.
  eventId: 12345 # The numeric ID of the event *type*
  provider: "54849625-5478-4994-a5ba-3e3b0328c30d" # The GUID of the event provider, e.g. "Microsoft-Windows-Security-Auditing"
```

The `policy` block is used to define the retention policy. Retention policies may be based on durations (e.g. keep 
events for 30 days), or on the number of events (e.g. keep only the last 1,000 events).

### Timestamp Policy
Durations are specified using a string with a number and a unit, for example, "30d" for 30 days, "96h" for 96 hours, etc.
```yaml
policy:
  type: "timestamp"
  retentionPeriod: "30d" # Retain events for 30 days
```

### Count Policy
Count / volume policies act as *buffers* for events that do not match any other policy. For example, if policies are
defined to keep events for 30 days, but also keep "security" channel events for 90 days, and a count policy is defined
to keep the last 1,000 events, then all events registered within the last 30 days will be kept, all events in the
"security" channel within the last 90 days will be kept, alongside the *most recent* 1,000 events that fall outside of
those policies will be kept.

Moreover, count policies may be defined to apply to the entire event stream, or to a specific principal (i.e. computer).
In simpler terms, this means that you may define a policy to "keep the last 1,000 events", or "keep the last 1,000
events received from each computer". This is useful for ensuring that a minimum number of events are kept for each
computer, even if the computer does not generate many events. This policy is defined using the `applyTo` field, which
may be set to `global` or `principal`.

Only one `count` policy may be defined, as multiple `count` policies would be contradictory.

```yaml
policy:
  type: "count"
  applyTo: "global"
  volume: 1000 # Retain the last 1,000 events
```

```yaml
policy:
  type: "count"
  applyTo: "principal"
  volume: 1000 # Retain the last 1,000 events per principal
```

## Example Policies
### Keep events for 30 days
```yaml
filters:
  - label: "Minimum 30 days" # Human-readable label
    policy:
      type: "timestamp"
      retentionPeriod: "30d"
```

### Keep only the last 1,000 events
The `count` policy works as a *minimum* retention policy. If there are exceptions (e.g. keep for more than 30 days), it
is possible that more than 1,000 events will be retained.

```yaml
filters:
  - label: "Only 1000"
    policy:
      type: "count"
      group: "global"
      volume: 1000
```

### Keep last 1,000 events per principal (computer), with a minimum of 30 days retention
```yaml
filters:
  - label: "Last 1000 per computer"
    policy:
      type: "count"
      applyTo: "principal"
      volume: 1000
  - label: "Minimum 30 days"
    policy:
      type: "timestamp"
      retentionPeriod: "30d"
```

### As above, but keep events in the "security" channel for 90 days
```yaml
filters:
  - label: "Last 1000 per computer"
    policy:
      type: "count"
      applyTo: "principal"
      volume: 1000
  - label: "Security channel 90 days"
    match:
      channel: "security"
    policy:
      type: "timestamp"
      retentionPeriod: "90d"
  - label: "Minimum 30 days"
    policy:
      type: "timestamp"
      retentionPeriod: "30d"
``` 

### As above, but only keep events with ID 12345 in the security channel for 90 days
```yaml
filters:
  - label: "Last 1000 per computer"
    policy:
      type: "count"
      group: "principal"
      volume: 1000
  - label: "Security channel 90 days"
    match:
      channel: "security"
      eventId: 12345
    policy:
      type: "timestamp"
      retentionPeriod: "90d"
  - label: "Minimum 30 days"
    policy:
      type: "timestamp"
      retentionPeriod: "30d"
```

### Keep events from the "Microsoft-Windows-Security-Auditing" event provider for 90 days, plus an additional 10,000 events not matching the provider
Providers are matched [GUID](https://learn.microsoft.com/en-us/windows/win32/api/guiddef/ns-guiddef-guid). The GUID for 
"Microsoft-Windows-Security-Auditing" is `54849625-5478-4994-a5ba-3e3b0328c30d`.

```yaml
filters:
  - label: "Windows Security Audit 90 days"
    match:
      provider: "54849625-5478-4994-a5ba-3e3b0328c30d"
    policy:
      type: "timestamp"
      retentionPeriod: "90d"
  - label: "Last 10000"
    policy:
      type: "count"
      volume: 10000
```
