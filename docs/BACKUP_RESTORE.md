# Backup and restore

ServerWatch stores everything in a single PostgreSQL database: monitor
definitions, check history (`monitor_results`), alert state and history, groups,
and (when account authentication is enabled) users and sessions. Backing up that
database is sufficient to recover the application; the binary and built
dashboard can be reinstalled from the repository.

## What to back up

- **The database** — the important part. Use `scripts/backup.sh`.
- **The environment file** (`/opt/serverwatch/serverwatch.env`) — contains the
  database password, API token, SMTP credentials, and auth settings. It is not
  in the database and is not committed to git. Store it in your secrets manager
  or a protected location. Losing it is recoverable (you can reconfigure), but
  you will need the same `POSTGRES_PASSWORD` to read an old dump conveniently.
- **Nothing else is required.** `web/dist` is a build artifact.

## Backup

```sh
scripts/backup.sh [BACKUP_DIR]
```

The script reads `POSTGRES_DB`/`POSTGRES_USER` from the environment file,
`pg_dump`s the `serverwatch-postgres` container in custom format, gzips it to
`serverwatch-<UTC-timestamp>.dump.gz` (default directory
`/var/backups/serverwatch`), and prunes everything but the most recent 7 files.

Relevant environment overrides:

| Variable | Default | Purpose |
| --- | --- | --- |
| `SERVERWATCH_BACKUP_DIR` | `/var/backups/serverwatch` | Output directory |
| `SERVERWATCH_BACKUP_KEEP` | `7` | Number of backups to retain |
| `SERVERWATCH_ENV_FILE` | auto-detected | Path to the env file |
| `SERVERWATCH_POSTGRES_CONTAINER` | `serverwatch-postgres` | Container name |

### Schedule it

Example daily cron entry (02:30 UTC) for the `serverwatch` user:

```cron
30 2 * * * SERVERWATCH_BACKUP_DIR=/var/backups/serverwatch /opt/serverwatch/scripts/backup.sh >> /var/log/serverwatch-backup.log 2>&1
```

Or a systemd timer if you prefer. Verify the first run manually and check that
the dump is non-empty.

## Restore

```sh
sudo systemctl stop serverwatch
scripts/restore.sh /var/backups/serverwatch/serverwatch-YYYYMMDDTHHMMSSZ.dump.gz
sudo systemctl start serverwatch
```

The script confirms interactively (pass `--yes` to skip), stops the service
best-effort, and runs `pg_restore --clean --if-exists` so it is safe against a
database that already contains data. The restore **overwrites** current
contents.

`pg_restore` prints warnings for objects it skips (for example, roles owned by
another user when restoring with `--no-owner`); these are expected. A non-zero
exit means at least one statement failed — inspect the output before starting
the service.

## Verifying a backup

Restore into a scratch database rather than the live one to confirm a dump is
readable:

```sh
docker exec serverwatch-postgres createdb -U serverwatch serverwatch_verify
gunzip -c serverwatch-YYYYMMDDTHHMMSSZ.dump.gz \
  | docker exec -i serverwatch-postgres pg_restore -U serverwatch -d serverwatch_verify --no-owner
docker exec serverwatch-postgres psql -U serverwatch -d serverwatch_verify \
  -c "SELECT count(*) FROM monitors; SELECT count(*) FROM monitor_results;"
docker exec serverwatch-postgres dropdb -U serverwatch serverwatch_verify
```

## Notes

- Backups are consistent snapshots taken with `pg_dump`; the database can stay
  online. There is a tiny window before a `pg_dump` transaction starts during
  which a check result may not be included, which is harmless.
- Consider copying backups off-host; a backup that lives only on the same disk
  as the database does not protect against disk failure.
- Data removed by the [retention job](CONFIGURATION.md#data-retention) is not in
  recent backups either. If you need long-term history, keep older backups or
  export to a separate store before the retention window closes.
