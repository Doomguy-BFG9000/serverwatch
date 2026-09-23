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

## Monitor group issues

Group names are case-insensitively unique. Creating or renaming a group to a
name that differs only by letter case from an existing group returns a conflict
instead of changing the existing group.

A monitor becoming ungrouped after a group is deleted is expected. Deleting a
group does not delete any monitor, and the monitor's checks continue normally.
Use the dashboard's `Ungrouped` filter to find those monitors and assign them
to another group if needed.

Groups do not pause monitors and do not affect scheduler, checker, or alert
behavior. If a monitor's status is stale after editing its group, group
assignment itself does not trigger an immediate check; wait for the next
configured monitoring interval.

## Monitor search returns no results

The dashboard monitors list is filtered by the selected group first and then by
the search text, so a monitor can be hidden either by the group filter or by
the search. If every monitor disappears, check the Group control as well as the
search box, and clear the search text to see the full group again. Search is
case-insensitive and matches monitor names and targets only; it does not search
groups, status, or history.

`No monitors match your search.` means monitors exist but none match the
current search (and group). `No monitors configured.` means no monitors exist
at all. Search and sorting are display-only and never delete, pause, or modify
monitors, and the selected search, sort, and group are not remembered after a
browser reload.

## Maintenance mode

Maintenance mode is manual and per-monitor. Entering maintenance pauses
scheduled checks for that monitor, so no new scheduled result is written and no
scheduled DOWN, recovery, or SSL-expiry alert is sent while it is active. The
monitor, its configuration, its group, its check history, and its alert state
are all preserved.

A monitor in maintenance may still show its last historical status, and a
disabled monitor takes presentation precedence over maintenance. If a status
looks stale, that is expected: maintenance is operational state, not a
synthetic check result, and ServerWatch does not fabricate an UP result or a
recovery event.

Exiting maintenance does not run a check immediately. The monitor becomes
eligible for the scheduler again, and the next real scheduled check is
processed with the existing alert state, so it may produce a normal DOWN or
recovery alert. Maintenance has no effect on monitor groups or group filtering,
and on-demand SSL/DNS/Port diagnostic panels still run a fresh display check
when opened. Maintenance does not create any alert transition by itself.
