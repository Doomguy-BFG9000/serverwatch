
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
