# ServerWatch Agent

> **Status: placeholder.** In Community Edition 0.1.0 the agent at `agent/` is a
> stub entrypoint. It does not report host metrics yet. Server, website, port,
> DNS, and SSL monitoring all run from the central backend and do **not** require
> the agent. See [ROADMAP.md](ROADMAP.md) for agent and host-metric plans.

## Why an agent

The central backend already checks services reachable over the network. Agents
exist for the things a server must measure from the inside: CPU, memory, disk,
load average, and uptime, plus local cron heartbeats and process/container
state. Those need code running on the monitored host.

## Current state

- `agent/cmd/serverwatch-agent/main.go` prints a startup line and exits.
- There is no agent protocol, registration, or metrics schema yet.

## What will not change

- The central backend keeps working without any agent installed.
- Network monitors (HTTP/TCP/ping/DNS/SSL/ports) stay on the backend.
- When the agent ships, enrollment will be opt-in per host.

## Contributing

Agent and host-metric work is tracked in [ROADMAP.md](ROADMAP.md). If you want to
work on it, open an issue to agree on the protocol and metrics schema before
writing code, so the backend, agent, and dashboard land together.
