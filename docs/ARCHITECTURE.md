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
