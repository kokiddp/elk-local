# Backup Workflows

ELK-Local supports backup, export, import, and restore from both the CLI and the local dashboard.

## Commands

```bash
go run ./cmd/elk-local backup my-site
go run ./cmd/elk-local backup my-site --include-project-files
go run ./cmd/elk-local export my-site --output /tmp/my-site.tar.gz --include-project-files
go run ./cmd/elk-local import my-site /tmp/my-site.tar.gz
go run ./cmd/elk-local restore my-site my-site-20260328-123456Z.tar.gz --force
go run ./cmd/elk-local restore my-site /tmp/my-site.tar.gz --project-files --force
```

## Archive Format

Backup and export both produce the same portable `.tar.gz` format:

- `metadata.json`: archive metadata including source environment and whether project files were included.
- `manifest.yaml`: a copy of the environment manifest at backup time.
- `database.sql`: a database dump captured through the environment's `db` service.
- `project/`: an optional project snapshot when `--include-project-files` is used.

## Behavior

- `backup` writes into `storage.backupsPath` for the target environment.
- `export` writes the same archive format to a path you choose.
- `import` copies a portable archive into the environment's managed `backups/` directory.
- `restore` resolves either a direct archive path or a file name inside the managed `backups/` directory.
- The dashboard lists managed archives per environment and can download, open the containing folder, restore, or delete a selected managed archive directly.

## Safety Notes

- `restore` requires `--force` because it replaces the target database contents.
- `restore --project-files` overwrites matching files from the archive but does not delete extra files already present in the project root.
- Project snapshots intentionally skip the top-level `.elk-local/` directory so managed environment artifacts do not get recursively backed up.

## Current Limits

- Restore currently targets the configured database for the selected environment; it does not recreate or retarget environments automatically.
- Delete only removes the managed archive file from the environment inventory. It does not touch exported copies stored elsewhere.