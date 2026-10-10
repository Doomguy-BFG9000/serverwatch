
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

## Dashboard search and sorting

The dashboard monitor list can be searched and reordered from the controls
next to the group filter. Both features are client-side and operate only on the
monitors already loaded by the browser; they add no API endpoint, database
query, or migration.

Search is case-insensitive and matches a monitor's name or target. Leading and
trailing whitespace in the typed text is ignored, and an empty search behaves
as no filter. Search composes with the group filter in a fixed order: the
loaded monitors are first limited by the selected group (All Groups, Ungrouped,
or a specific group), then filtered by the search text, and finally sorted.
When monitors exist but none match, the dashboard shows a no-match message
instead of the empty "no monitors configured" state, and it does not clear the
search or change the selected group.

The Sort control offers Default, Name A → Z, Name Z → A, Status, Monitor type,
Newest, and Oldest. Default preserves the order the dashboard already used
before search and sorting existed. Name sorting is case-insensitive; Monitor
type sorting is case-insensitive ascending by `monitor_type`; Newest and Oldest
use the monitor's `created_at` timestamp. Status sorting reuses the dashboard
presentation state, where Disabled outranks Maintenance, which outranks the
normal monitoring status, and orders monitors Down, Up, Pending/Unknown,
Maintenance, then Disabled. Equal values fall back to monitor name and then
monitor ID so ordering is deterministic.

Search and sorting never modify a monitor record and do not affect intervals,
timeouts, checker behavior, scheduler dispatch, alerting, maintenance, groups,
or history. Search and sort selections are not persisted across a browser
reload.

## Dashboard health summary

Above the monitor controls the dashboard shows a health summary of six counts:
Total Monitors, Up, Down, Maintenance, Disabled, and Pending / No Data. It is
client-side and derived from the monitors the browser has already loaded, so it
adds no API endpoint, database query, or migration, and no new dependency.

Each configured monitor is counted in exactly one bucket, using the same state
precedence as the rest of the dashboard. A disabled monitor is counted as
Disabled even when it is also in maintenance or its last result was DOWN. A
monitor that is enabled and in maintenance is counted as Maintenance no matter
what its last result was. Otherwise the monitor's current status decides the
bucket: Up, Down, or Pending / No Data when there is no result yet. The buckets
partition the monitors, so Up + Down + Maintenance + Disabled + Pending equals
Total Monitors at all times.

The counts cover all configured monitors and are intentionally independent of
the Group filter, Search, and Sort controls, which change only the monitor
cards displayed below the summary. The six cards are informational and are not
clickable, so the dashboard keeps a single filtering pipeline (group, then
search, then sort) without an extra status dimension. When there are no
monitors, every count is zero and the summary is still shown.

## Maintenance mode

Maintenance mode is a manual, per-monitor switch for planned work. It is
separate from the `enabled` flag:

- `enabled: false` administratively disables the monitor.
- `enabled: true` with `maintenance: true` keeps the monitor configured but
  pauses scheduled monitoring.
- `enabled: true` with `maintenance: false` is normal monitoring.

The dedicated endpoint toggles the flag:

```text
PUT /api/v1/monitors/{id}/maintenance
```

```json
{ "maintenance": true }
```

Send `"maintenance": false` to exit. The response is the updated monitor
representation. A malformed monitor id or malformed JSON returns `400`, a
missing `maintenance` field or a non-boolean value returns `400`, and an unknown
monitor returns `404`. Setting the same value repeatedly is safe.

While maintenance is active, scheduled checks are paused, so no new scheduled
result is written and no scheduled DOWN, recovery, or SSL-expiry alert is sent.
Existing check history and alert state are preserved. Exiting maintenance does
not run a check immediately and does not fabricate an UP result; the monitor
becomes eligible for the scheduler again and the next real scheduled check is
processed with the preserved alert state. The monitor's group is unaffected.

The general monitor `PUT` endpoint also accepts an optional `maintenance`
boolean. Omit it to leave the current value unchanged, or send `"maintenance":
true` / `"maintenance": false` to change it explicitly. The dashboard uses the
dedicated endpoint with a confirmation instead of editing it in the monitor
form, and maintenance is not a monitor-creation field (new monitors default to
`maintenance: false`).

Maintenance is manual and per-monitor. There are no schedules, start/end times,
recurrence, or reasons in this phase. On-demand diagnostic panels for
SSL/DNS/Port monitors still run a fresh check when opened; that is a display
action and does not create a scheduled result or an alert transition.

## API access token

The ServerWatch API is open by default. Setting `SERVERWATCH_API_TOKEN` to a
non-empty value turns on shared-secret bearer authentication. The setting is
read once at startup, so change it in the environment file and restart the
service:

```dotenv
SERVERWATCH_API_TOKEN=<secret>
```

Generate a strong value and never commit it:

```sh
openssl rand -hex 32
```

While a token is configured:

- requests to `/api/v1/...` must include `Authorization: Bearer <secret>`;
  a missing or wrong token returns `401` with `{"error":"unauthorized"}` and a
  `WWW-Authenticate: Bearer` header;
- the comparison is constant-time and the scheme is case-insensitive
  (`Bearer`/`bearer` both work), but the token itself is compared exactly;
- `/health` and `/ready` are always open so liveness/readiness probes, systemd,
  Docker health checks, and Nginx keep working;
- when the variable is empty or unset, the middleware is a pass-through and the
  API is open exactly as before.

The dashboard prompts for the token when it receives a `401`, stores it in the
browser's `localStorage` (key `serverwatch_api_token`), and sends it on every
request. Use the top-bar **API Token** button to set or clear it. Clearing the
token and reloading returns the browser to unauthenticated access.

This is a single shared secret for the whole installation; it is not a multi-user
account system, has no roles, and does not provide an audit trail of who made a
change.

## Account authentication

Setting `SERVERWATCH_AUTH_ENABLED=true` protects the dashboard and API with a
login. It is read once at startup, so change it in the environment file and
restart the service:

```dotenv
SERVERWATCH_AUTH_ENABLED=true
SERVERWATCH_SESSION_TTL=720h
SERVERWATCH_COOKIE_SECURE=false
```

- `SERVERWATCH_SESSION_TTL` is a Go duration for how long a login lasts
  (default `720h`, 30 days).
- `SERVERWATCH_COOKIE_SECURE` adds the cookie `Secure` attribute. Leave it
  `false` for plain-HTTP LAN access; set it `true` when the dashboard is served
  over HTTPS.

When authentication is enabled and no account exists yet, `GET
/api/v1/auth/status` reports `setup_required: true` and the dashboard shows a
first-run form. `POST /api/v1/auth/setup` creates the administrator only while
the users table is empty; once an account exists it returns `409` and can never
be used to add users or take over the installation. Afterwards, `POST
/api/v1/auth/login` starts a session, `POST /api/v1/auth/logout` ends it, and
`GET /api/v1/auth/me` returns the current user.

Sessions are stored as SHA-256 hashes of 256-bit random tokens, delivered in an
HttpOnly, SameSite=Lax cookie (add `Secure` via `SERVERWATCH_COOKIE_SECURE`).
Passwords are stored as PBKDF2-HMAC-SHA256 hashes (210,000 iterations, random
16-byte salt) and verified in constant time.

While authentication is enabled, the middleware accepts **either** a valid
session cookie **or** a valid `SERVERWATCH_API_TOKEN` bearer token, so scripts
and health checks keep working. `/health`, `/ready`, and the four `/api/v1/auth/*`
entry points are always reachable; every other `/api/v1/...` route returns `401`
without a valid session or token.

This is a single-administrator model. There are no per-user roles, no
multi-tenant permissions, and no audit log of individual changes; adding those
is future work.
