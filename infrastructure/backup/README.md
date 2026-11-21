# Backup and restore (Story 1.22, ARCHITECTURE AD-22)

What is backed up, how, and what is **not** promised. Everything below is exercised by `tests/ops/test_backup.sh` (real MySQL 8.0 with binlog, a second real `mysqld` as the restore target, the real binary's encryption) and `cmd/api/imbackup_test.go` (real WuKongIM process), on a development machine. None of it replaces the restore drill on a real second host (`docs/ops/MANUAL-CHECKLIST.md` C10).

## The model
| Data | Method | Data-loss window (RPO) |
|---|---|---|
| MySQL (business state, `message_ref`, audit log) | daily full logical dump (`mysqldump --single-transaction`, no writer locks) **plus** closed binlog files shipped every 5 minutes → point-in-time recovery | up to the binlog interval (5 min by default) **if the host dies**: the active binlog file is not shipped until it closes. A mistaken `DELETE` is recoverable to the second, because the log up to the accident has been shipped |
| WuKongIM data directory (message bodies) | stop the IM, copy its data directory locally (seconds), start it, then encrypt and ship the copy | **the snapshot interval (daily by default)**. Messages sent since the last snapshot are lost if the IM's disk is lost. See "IM" below |
| Redis | not backed up: it holds only reconstructible state (rate counters, the token deny-list). Losing it un-revokes revoked tokens until they expire (≤ 15 min) | — |
| Object storage (media) | provider versioning + lifecycle (`object-storage-setup.sh`) | provider's |

RPO and RTO targets are **configuration** (PRD OQ-7), not claims. The numbers above are what the defaults give; the restore time is only known from a recorded drill on your hardware (`restore.sh` prints per-phase timings; `manifest` records IM downtime).

## Encryption and who can read what
Every artifact is encrypted **on the backup host** to an RSA public key before it is written. The private key is made on your own machine (`veyrogram-api backup keygen -out-dir D`), never copied to the backup host, and kept in at least two safe places: **without it no backup can be read, by anyone.** A host that is compromised leaks neither past nor future backups. Per-file random AES-256-GCM key, chunked, with position and end-of-file bound into each chunk: a flipped bit, a swapped, removed or appended chunk, a truncated file or the wrong key all fail (tested), and restore decrypts and authenticates **everything before touching the target**. Files are 0600 in a 0700 directory. The shipped copy goes off the host through `VG_BACKUP_UPLOAD_CMD`; a run without one **fails** by default.
The manifest next to each artifact is plain text and **not authenticated**: restore uses it only to detect damage in transit. The binlog position and database name are read from inside the authenticated dump.
Use upload credentials that can add objects but not delete or overwrite them, so a compromised host cannot erase its own history.

## MySQL
* Account: `veyrogram-api db provision-backup` (read-only; global privileges `SELECT, SHOW VIEW, TRIGGER, EVENT, RELOAD, REPLICATION CLIENT, REPLICATION SLAVE`; tested that it cannot write, alter, grant or change globals). Put it in a 0600 my.cnf (`VG_BACKUP_MYSQL_DEFAULTS`); the script refuses a password file others can read.
* Server settings (the install preflight reports them): binary logging on, `binlog_format=ROW`, MySQL 8.0.26 or newer (`--source-data`).
* Restore: `restore.sh mysql -d <dir> -k private.pem -t target.cnf [-u 'YYYY-MM-DD HH:MM:SS']` — **`-u` is UTC.** Picks the newest full not after `-u`, replays binlogs after it up to `-u`. Refuses a gap in the binlog chain and a target database that already has tables.
* Known limit: binlog replay filters with `mysqlbinlog --database`, which for statement-based events looks at the connection's **default database**. The application always connects with its database selected, so its DDL and DML are replayed; a DBA statement like `DROP TABLE veyrogram.x` typed in a session with no database selected is **not** replayed. Run manual DDL with `USE veyrogram` first.
* Keep a database server for VeyroGram's databases only; the binlog contains every database on the server and is shipped whole (encrypted).

## WuKongIM
Procedure (the only one verified): `backup.sh im` with `VG_IM_QUIESCE=stop`.
1. stop `veyrogram-wukongim` (the unit runs as the IM account; the polkit rule allows exactly that account to stop/start exactly that unit);
2. `cp -a` the data directory to local staging (this is the whole outage; recorded as `downtime_seconds` in the manifest; clients reconnect and nothing already acknowledged is lost, because the IM is simply stopped);
3. start it again (an exit trap guarantees this even if the copy fails);
4. tar, compress, encrypt and ship the copy while the IM is already serving.
The rehearsal stops the process hard (SIGKILL), copies, restores into a new directory, starts a new IM on it and checks that all 25 messages are served with identical sequence numbers and payloads, and that a new message continues the sequence.
`VG_IM_QUIESCE=none` is for operators who freeze the filesystem or take a volume snapshot (LVM/ZFS/btrfs) themselves and mount it at the data directory: nothing is stopped. **That path is not rehearsed here**; copying the files of a *running* IM without a snapshot is not claimed to be safe.
Restore: `restore.sh im -f <artifact> -k private.pem -o <empty dir>`, then start the IM against `<dir>/data`.
**After restoring both stores to different moments** (database to T, IM to the last snapshot) they disagree: `message_ref` rows can point at IM messages that do not exist, or the IM can hold messages with no `message_ref`. The reconciliation (rebuild `message_ref` from the IM's sequence numbers, which are authoritative) is **not implemented yet**; until it is, a cross-store restore needs a hand-check (checklist C10).

## Object storage
`object-storage-setup.sh <backup|media> <bucket>` enables versioning and applies a lifecycle policy to any S3-compatible bucket, then reads both back and fails if the provider reports otherwise. The retention numbers in `object-storage/*.json` are defaults to edit (OQ-7). Tested against a fake provider only.

## Monitoring
`veyrogram_backup_last_success_timestamp_seconds`, `…_last_failure_…`, `…_last_size_bytes` per kind (`mysql-full`, `mysql-binlog`, `im`), written for node_exporter's textfile collector. Alerts: `VeyroGramBackupFailed` (latest run failed), `VeyroGramBackupStale` (no success: 26 h for full/IM, 20 min for binlog), `VeyroGramBackupMetricsMissing` (no success metric at all: assume there is no backup). Unit tests in `veyrogram-alerts.test.yml`.

## Units
`veyrogram-backup-mysql-full.timer` (03:17), `-mysql-binlog.timer` (every 5 min), `-prune.timer` (05:41), `-im.timer` (04:07). Installed by `install.sh`, **not enabled** until you have set `/etc/veyrogram/backup.env` and `backup-im.env`, installed the public key and tested an upload. Enable with `systemctl enable --now <timer>`.
