
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
