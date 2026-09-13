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
