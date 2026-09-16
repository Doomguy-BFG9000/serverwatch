# Troubleshooting

## Ping monitor is DOWN but manual ping works

Ping monitoring uses the system `ping` command and does not require
ServerWatch to run as root. On Linux/Ubuntu, install the prerequisite with:

```sh
sudo apt install -y iputils-ping
```

Run these checks as the `serverwatch` service user where applicable:

1. Check that `ping` exists:

```sh
command -v ping
```

2. Test normal interactive ping:

```sh
ping -c 1 127.0.0.1
```

3. Inspect the executable's capabilities:

```sh
getcap "$(command -v ping)"
```

4. Inspect the unprivileged ICMP group range:

```sh
sysctl net.ipv4.ping_group_range
```

5. Reproduce the systemd `NoNewPrivileges` behavior:

```sh
setpriv --no-new-privs /usr/bin/ping -n -c 1 -W 5 127.0.0.1
```

The expected result is a successful ICMP reply and exit code `0`. If normal
`ping` works but this `setpriv` test fails with `Operation not permitted`,
`/usr/bin/ping` is likely relying on `cap_net_raw=ep`, which
`NoNewPrivileges=true` prevents it from acquiring. The configured
`net.ipv4.ping_group_range` likely does not include the effective group ID of
the ServerWatch service account.

Configure the range for the local `serverwatch` GID as described in
[the Ping monitoring architecture decision](ARCHITECTURE.md#unprivileged-icmp-with-ping_group_range):

```sh
SERVERWATCH_GID=$(id -g serverwatch)
sudo sysctl -w net.ipv4.ping_group_range="$SERVERWATCH_GID $SERVERWATCH_GID"
```

Use the persistent `/etc/sysctl.d/99-serverwatch-ping.conf` configuration from
the architecture document for the setting to survive reboot. Do not remove
`NoNewPrivileges=true`, run ServerWatch as root, or add `CAP_NET_RAW` to the
ServerWatch application.

## Port monitor shows DOWN

A `port` monitor reports `DOWN` when at least one configured port does not
accept a TCP connection. The result message names the unreachable ports, and the
monitor detail view shows per-port status, per-port latency, and a short reason
such as `connection refused`, `timeout`, `DNS resolution failed`,
`host unreachable`, or `unreachable`.

Common causes:

- the service is not listening on that port;
- a firewall blocks the connection;
- the host is unreachable from the ServerWatch server;
- the configured port number is wrong (for example `8080` instead of `80`);
- the connection timed out because the host silently drops packets;
- the hostname could not be resolved.

Check from the ServerWatch host:

```sh
getent hosts example.com
nc -vz example.com 22
```

Port monitors need no shell, no external binary, no root access, and no Linux
capability, so no installation step is required for this monitor type.

A port being reachable only means a TCP connection could be established. It
does not prove application-level correctness or that the service returns
correct responses. For HTTP application behavior, use HTTP/HTTPS monitoring.
