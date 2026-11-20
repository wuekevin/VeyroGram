# MySQL accounts (Story 1.4)

Three accounts, never shared:

| Account | Used by | Rights |
|---|---|---|
| admin | operator, one-off | creates the other two |
| migrator | `veyrogram-api migrate ...` | DDL on the application database |
| app | API and workers (`VG_MYSQL_DSN`) | table-level DML; **`audit_log`: SELECT, INSERT only** |

MySQL cannot subtract a table privilege from a database-level grant, so the app account is granted **per table**. After every migration that adds a
table, re-run (as admin):

    veyrogram-api db grant-app --database veyrogram --user vg_app --host 127.0.0.1

It is idempotent. Tables added by later migrations are NOT usable by the app until this has been run, which is the safe default.

## Provisioning (one-time, and for password rotation)
```
export VG_MYSQL_ADMIN_DSN='root:<admin-pass>@tcp(127.0.0.1:3306)/'        # admin only for these two commands
export VG_PROVISION_APP_PASSWORD=<>=24 chars>  VG_PROVISION_MIGRATOR_PASSWORD=<different, >=24 chars>   # from a root-only file, never argv
veyrogram-api db provision --database veyrogram --app-user vg_app --migrator-user vg_migrator --host 127.0.0.1
VG_MYSQL_DSN='vg_migrator:...@tcp(127.0.0.1:3306)/veyrogram' veyrogram-api migrate up
veyrogram-api db grant-app --database veyrogram --user vg_app --host 127.0.0.1
```
`provision` is idempotent; re-running with new passwords rotates them (the old ones stop working — covered by test). The app account gets no
database-level grant; `grant-app` gives per-table rights and keeps `audit_log` append-only.
