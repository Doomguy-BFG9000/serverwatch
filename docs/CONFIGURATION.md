
## SMTP email alerts

Email alerts are disabled unless explicitly enabled. Configure the ServerWatch
backend with environment variables:

```dotenv
SERVERWATCH_ALERT_EMAIL_ENABLED=true
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USERNAME=monitor@example.com
SMTP_PASSWORD=<secret>
SMTP_FROM=monitor@example.com
SMTP_TO=admin@example.com
SMTP_TLS=true
```

Never commit `SMTP_PASSWORD`. Apply database migration
`database/migrations/002_alerting.sql` before starting the backend.
## DNS monitoring

ServerWatch Community Edition supports DNS monitors for `A`, `AAAA`, `CNAME`, `MX`, `NS`, and `TXT` records. The target is a DNS name only, such as `example.com`: do not include a protocol, port, or path. A trailing FQDN dot is accepted and normalized.

An optional expected value can verify a returned record. For example:

- A: target `example.com`, expected value `203.0.113.10`
- MX: target `example.com`, expected value `mail.example.com`
- TXT: target `example.com`, expected value `verification-string`

Resolution uses the operating system's configured DNS resolver through Go's standard library. Custom DNS server selection is not part of Phase 0.35. DNS checks require no external DNS binary, shell, privilege, or Linux capability.

### Check timing after monitor edits

Monitor configuration changes are saved immediately, but the displayed UP/DOWN status is based on the latest completed scheduled check. Editing DNS Expected Value, DNS Record Type, the target, or another check setting does not itself start a new check, so the previous status may remain visible until the next scheduled check. The normal wait is approximately one configured monitoring interval, subject to scheduler timing.

For example, with a 60-second interval, changing an Expected Value at 22:23 may leave the previous status visible until a scheduled check around 22:24. This monitoring interval is separate from response time: a monitor may take about 60 seconds to reflect an edit while the DNS lookup itself takes only 5 ms. Live DNS validation confirmed that status transitions and DOWN/RECOVERY alerts followed subsequent scheduled checks; this delay is expected behavior, not DNS lookup latency.

## Multi-port monitoring

ServerWatch Community Edition supports `port` monitors that check several
explicitly configured TCP ports on one host. The target is one hostname or IP
address only, such as `example.com`, `192.168.1.10`, `2001:db8::10`, or
`localhost`: do not include a protocol, path, query, fragment, or port. Ports
are configured separately in the monitor config:

```json
{
  "ports": [22, 80, 443]
}
```

Port rules:

- between 1 and 20 ports per monitor;
- every port is an individual whole number from 1 to 65535;
- ranges such as `22-80` or `1-65535` are rejected;
- duplicates are normalized into a single entry and stored sorted ascending, so
  `443, 80, 443, 22` is stored as `22, 80, 443`.

A port monitor is `UP` only when every configured port accepts a TCP
connection. If any configured port is unreachable, the monitor is `DOWN` and
the result message reports how many ports were reachable:

- `3/3 ports reachable`
- `2/3 ports reachable; port 443 unreachable`
- `1/3 ports reachable; ports 80, 443 unreachable`

`response_time_ms` is the wall-clock duration of the complete multi-port check,
not the sum of the individual port latencies. Per-port status and per-port
latency for a fresh check are available in the monitor detail view and from:

```text
GET /api/v1/monitors/{id}/ports
```

Checks use bounded concurrency: at most 5 TCP connection attempts run at the
same time, and the monitor's `timeout_seconds` is a single overall deadline for
the whole check instead of a per-port budget. Port monitors need no shell,
external binary, root access, or Linux capability; they use Go's standard TCP
networking.

### Check timing after monitor edits

Editing the Ports list, the target, or another check setting is saved
immediately, but the displayed UP/DOWN status still comes from the latest
completed scheduled check, so it can lag by roughly one monitoring interval.
ServerWatch does not run an immediate check after an edit, and the next
scheduled check uses the new port configuration and then evaluates DOWN and
RECOVERY alerts.

## Monitor groups

Monitor groups organize monitors in the dashboard. A monitor may belong to zero
or one group:

```json
{
  "name": "Production API",
  "monitor_type": "https",
  "target": "https://api.example.com",
  "group_id": 3
}
```

Send `group_id` as `null` or omit it when creating a monitor to leave the
monitor ungrouped. The group API is:

Monitor updates use `PUT`. When updating a monitor, omit `group_id` to leave
the current group unchanged, or send `"group_id": null` to explicitly remove
the monitor from its group.

```text
GET    /api/v1/groups
POST   /api/v1/groups
PUT    /api/v1/groups/{id}
DELETE /api/v1/groups/{id}
```

Group names are required, trimmed, limited to 100 characters, and unique
case-insensitively. Deleting a group does not delete its monitors; assigned
monitors become ungrouped.

Groups are organizational only. They do not change monitor intervals,
timeouts, checker behavior, scheduler dispatch, or alerting. Dashboard group
filtering only changes which monitors are visible.
