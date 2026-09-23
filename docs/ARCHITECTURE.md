# ServerWatch Architecture

## Ping monitoring

ServerWatch Community Edition Ping monitoring uses the operating system's
`ping` executable. The Go application does not implement raw ICMP sockets.
The monitor starts the executable with Go's `exec.CommandContext`:

- the target is passed as a separate command argument;
- no `sh -c` or other shell command construction is used; and
- the application timeout and cancellation control the `ping` subprocess.

This keeps ServerWatch non-root, avoids granting `CAP_NET_RAW` to the
ServerWatch application, and avoids implementing privileged raw-socket ICMP
code in the application.

The primary supported deployment environment for this implementation is Linux,
including Ubuntu. The runtime dependency is `iputils-ping`; ServerWatch does
not install it automatically:

```sh
sudo apt install -y iputils-ping
```

Ping monitor targets are hostnames or IP addresses. They are not shell input
and must not include a protocol, path, or port.

## Systemd security decision

The ServerWatch systemd service intentionally retains:

```ini
NoNewPrivileges=true
```

This setting must not be removed merely to make Ping monitoring work. On
systems where `/usr/bin/ping` relies on the file capability
`cap_net_raw=ep`, `NoNewPrivileges` can prevent the subprocess from acquiring
that capability. As a result, a Ping monitor can report `DOWN` even when
running `ping` manually from an interactive shell succeeds.

ServerWatch is not run as root, is not given `CAP_NET_RAW`, and is not given
`AmbientCapabilities=CAP_NET_RAW`. The application is not made privileged to
support Ping monitoring.

## Unprivileged ICMP with `ping_group_range`

The selected solution is Linux unprivileged ICMP socket support through
`net.ipv4.ping_group_range`. The configured range must include the effective
group ID of the account that runs ServerWatch.

GIDs differ between installations; do not copy a development server's GID.
An administrator can obtain the installed service account's GID and apply a
runtime setting as follows:

```sh
id -g serverwatch

SERVERWATCH_GID=$(id -g serverwatch)
sudo sysctl -w net.ipv4.ping_group_range="$SERVERWATCH_GID $SERVERWATCH_GID"
```

This is host runtime configuration, not a ServerWatch application setting.

### Persistent configuration

For persistence across reboots, determine the local GID and create a sysctl
drop-in. The value in the file must be replaced with that installation's
`SERVERWATCH_GID`:

```sh
SERVERWATCH_GID=$(id -g serverwatch)
sudo nano -l /etc/sysctl.d/99-serverwatch-ping.conf
```

The file should contain:

```text
net.ipv4.ping_group_range = <SERVERWATCH_GID> <SERVERWATCH_GID>
```

Replace both placeholders with the numeric value returned by `id -g
serverwatch`, then apply and verify the configuration:

```sh
sudo sysctl --system
sysctl net.ipv4.ping_group_range
```

## Validation

The following checks confirm the executable, its capabilities, the host ICMP
configuration, and ordinary interactive behavior:

```sh
command -v ping
getcap "$(command -v ping)"
sysctl net.ipv4.ping_group_range
ping -c 1 127.0.0.1
```

The important service-equivalent test is:

```sh
setpriv --no-new-privs /usr/bin/ping -n -c 1 -W 5 127.0.0.1
```

It should receive a successful ICMP reply and exit with code `0`. This
approximates the `NoNewPrivileges` restriction used by the ServerWatch
systemd service and verifies that Ping works without weakening that security
model. After configuring the host, restart the ServerWatch service through
the normal operational procedure and confirm that a local Ping monitor moves
from `DOWN` to `UP`.

## Multi-port monitoring

Port monitors check up to 20 explicitly configured TCP ports on one host using
Go's standard `net` package (`net.Dialer.DialContext`), reusing the same
connection helper as the plain TCP monitor. No shell, external binary, raw
socket, or privileged operation is involved, so port monitoring requires no
`CAP_NET_RAW`, no root, and no additional system package. Only ports the user
explicitly configured are dialed: no ranges, subnets, automatic discovery, or
service fingerprinting are part of this feature.

## Monitor groups

Monitor groups store organizational metadata in a separate `monitor_groups`
table. The `monitors.group_id` reference is nullable and uses
`ON DELETE SET NULL`, so deleting a group leaves every monitor intact and
ungrouped.

Groups are deliberately not part of the monitoring runtime. The scheduler's
due-monitor query does not join or filter on groups, monitor checkers do not
read group metadata, and alert transitions remain based only on monitor
results. Group filtering in the dashboard is client-side presentation logic.

## Dashboard search and sorting

Monitor search and sorting are frontend-only presentation features. The
dashboard derives the visible list from the monitors it has already loaded: it
applies the selected group filter, then the case-insensitive name/target
search, then the selected sort, and renders the result. No search or sort
endpoint exists, no SQL filtering is added, and no database index or migration
is required.

Sorting operates on a copied array so the loaded `monitors` state order is
never mutated, and each explicit sort uses explicit tie-breakers ending in
monitor ID for deterministic output. The default sort preserves the incoming
order from `GET /api/v1/monitors/status`, which is already ordered by monitor
ID descending. The status sort reuses the same presentation precedence as the
rest of the dashboard (Disabled, then Maintenance, then the stored status), so
search and sorting cannot change how a monitor's state is displayed.

Because Newest and Oldest sort on the monitor's `created_at` timestamp, the
status response exposes that field. This is additive and does not change any
API semantics, scheduler dispatch, checker behavior, or alert logic; the
timestamp is read-only data already stored on the monitor row.

## Monitor maintenance mode

Maintenance mode is stored as a single `monitors.maintenance BOOLEAN NOT NULL
DEFAULT FALSE` column (migration `005_monitor_maintenance.sql`). It is manual,
per-monitor operational state, not a synthetic check result: the toggle never
inserts a `monitor_results` row.

The scheduled-check pause is enforced where due monitors are selected. The
scheduler's due-monitor query filters on `enabled = TRUE AND maintenance =
FALSE`, so a monitor in maintenance is never dispatched to a checker and no
scheduled result or scheduled alert transition is generated for it. This keeps
the pause in the query instead of loading and discarding monitors after
dispatch, and it does not change scheduler cadence, concurrency, or checker
behavior.

The dedicated maintenance endpoint (`PUT /api/v1/monitors/{id}/maintenance`)
only writes the `maintenance` column. It does not run a checker, write a result,
or touch `monitor_alert_state`. Because alert transitions are produced only by
the scheduler's alert path, which is skipped while maintenance is active, no
DOWN, recovery, or SSL-expiry alert can be triggered by entering or leaving
maintenance. Exiting maintenance does not run a check immediately; the next
real scheduled check is evaluated with the preserved alert state.

The `enabled` flag and `maintenance` are independent persistent state. They are
never coupled: disabling a monitor does not clear maintenance, and removing
maintenance does not re-enable a disabled monitor. For presentation only, the
dashboard shows `Disabled` (Paused) before `Maintenance` before the stored
status.

On-demand diagnostic endpoints (`/ssl`, `/dns`, `/ports`) are unchanged and
remain callable while a monitor is in maintenance. They run a fresh check for
display but never mutate scheduler alert state or write scheduled results.

No scheduling windows, start/end times, recurrence, or maintenance reasons are
part of this phase; maintenance is a manual on/off flag only.
