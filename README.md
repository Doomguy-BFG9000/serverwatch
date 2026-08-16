# ServerWatch Community Edition

ServerWatch is a lightweight self-hosted monitoring platform for servers, websites, APIs, SSL certificates, cron jobs, and infrastructure health.

## Project Status

Early development / Community Edition.

## Initial Goals

- Central dashboard for multiple servers
- HTTP/HTTPS uptime monitoring
- SSL certificate expiry monitoring
- TCP port monitoring
- Cron heartbeat monitoring
- Lightweight ServerWatch agent
- CPU, RAM, disk, load, and uptime metrics
- Email and Telegram alerts
- Docker-based deployment
- Simple self-hosted installation

## Architecture

ServerWatch consists of:

- Go backend/API
- Go monitoring agent
- React + TypeScript frontend
- PostgreSQL database
- Docker Compose deployment

## Repository Structure

- `server/` - central ServerWatch backend
- `agent/` - lightweight monitored-server agent
- `web/` - React frontend
- `database/` - migrations and database-related files
- `deployments/` - deployment configuration
- `docs/` - project documentation
- `scripts/` - helper/install scripts
- `tests/` - integration and system tests

## License

Apache License 2.0
