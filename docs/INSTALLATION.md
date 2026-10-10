# Installation

ServerWatch Community Edition runs as a single Go backend plus a Postgres
container. The dashboard is static files served by Nginx (or any web server).

## 1. Unpack the release

```bash
tar xzf serverwatch-v0.1.0-linux-amd64.tar.gz
sudo mv serverwatch-v0.1.0-linux-amd64 /opt/serverwatch
cd /opt/serverwatch
```

The bundle contains the `bin/serverwatch` backend, the built dashboard under
`web/dist`, database migrations, deployment templates, docs, and backup scripts.

## 2. Configure

```bash
cp .env.example serverwatch.env
chmod 600 serverwatch.env
```

Set at minimum `DATABASE_URL`. See [CONFIGURATION.md](CONFIGURATION.md) for the
full reference, including SMTP and Telegram alerts, authentication, retention,
and metrics.

## 3. Start Postgres

```bash
docker compose --env-file serverwatch.env up -d postgres
```

The backend applies pending migrations from the embedded copy on startup, so no
manual step is required.

## 4. Run the backend

```bash
./bin/serverwatch
```

Confirm it is healthy:

```bash
curl -s http://127.0.0.1:8080/health   # {"status":"ok","version":"v0.1.0",...}
curl -s http://127.0.0.1:8080/ready    # {"status":"ok","database":"ok",...}
```

## 5. Install as a service

```bash
sudo cp deployments/systemd/serverwatch.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now serverwatch
```

The unit reads `/opt/serverwatch/serverwatch.env` and starts Postgres first.

## 6. Serve the dashboard

```bash
sudo cp deployments/nginx/serverwatch.conf /etc/nginx/sites-available/serverwatch
sudo ln -s /etc/nginx/sites-available/serverwatch /etc/nginx/sites-enabled/serverwatch
sudo nginx -t && sudo systemctl reload nginx
```

Nginx serves `web/dist`, proxies `/api`, and terminates TLS. See the HTTPS
section at the bottom of the Nginx template, and set `SERVERWATCH_COOKIE_SECURE=true`
once TLS works.

## Upgrading

1. Stop the service: `sudo systemctl stop serverwatch`.
2. Back up first: `scripts/backup.sh`.
3. Replace the contents of `/opt/serverwatch` (keep your `serverwatch.env`).
4. Start the service; migrations apply on startup.

## Verifying a release

```bash
sha256sum -c SHA256SUMS
```
