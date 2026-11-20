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
