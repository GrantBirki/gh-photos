# Audit Trail & Manifest System

## Overview

The `gh-photos` CLI now includes a comprehensive **audit trail and manifest system** that provides:

- **Audit Trail**: Permanent record of every successful run with complete metadata
- **Repeatability**: Re-run the last successful command using `--use-last-command`
- **Transparency**: Detailed per-asset tracking with status, metadata, and statistics
- **Extensibility**: JSON format designed for future enhancements

## File Layout

The CLI manages a directory at `~/gh-photos/`:

```text
~/gh-photos/
├── manifest.json                      # Latest successful run (symlink/copy)
├── manifest_2025-09-18T10-45-00Z.json # Timestamped permanent record
├── manifest_2025-09-17T14-22-15Z.json # Previous runs...
└── manifest_2025-09-16T09-33-42Z.json
```

- **`manifest.json`** - Always reflects the **last successful run**
- **Timestamped manifests** - Immutable permanent audit records  
- **RFC3339 format** - Consistent timestamp naming (`manifest_<timestamp>.json`)

## Manifest Structure

Each manifest contains comprehensive metadata:

### Example Manifest

```json
{
  "metadata": {
    "run_id": "2025-09-18T10:45:00Z",
    "cli_version": "0.1.0",
    "device": {
      "backup_path": "/Users/birki/Apple/MobileSync/Backup/00008110-000939D83484801E",
      "device_name": "Birki's iPhone",
      "device_uuid": "00008110-000939D83484801E",
      "ios_version": "17.6"
    },
    "invocation": {
      "remote": "gdrive:Backups/iPhone/photos",
      "flags": {
        "include_hidden": false,
        "include_recently_deleted": false,
        "parallel": 4,
        "skip_existing": true,
        "dry_run": false,
        "log_level": "debug",
        "types": ["photo", "video", "screenshot", "burst", "live_photo"],
        "start_date": null,
        "end_date": null
      }
    },
    "summary": {
      "assets_total": 4120,
      "assets_uploaded": 87,
      "assets_skipped": 4033,
      "assets_failed": 0,
      "bytes_transferred": 935184230,
      "duration_seconds": 64.2
    },
    "system": {
      "os": "darwin",
      "hostname": "Birki-MacBook-Pro",
      "arch": "arm64"
    }
  },
  "assets": [
    {
      "uuid": "4BFEF3E3-12E9-4C19-9D1E-CC9F4B0B824A",
      "local_path": "/.../DCIM/100APPLE/IMG_0012.HEIC",
      "remote_path": "photos/2025/09/18/photo/IMG_0012.HEIC",
      "size_bytes": 2048932,
      "sha256": "abc123...",
      "type": "photo",
      "hidden": false,
      "deleted": false,
      "created_at": "2025-09-18T08:12:04Z",
      "status": "uploaded"
    }
  ]
}
```

## CLI Usage

### New Flags

#### `--save-audit-manifest <path>`

Save an additional copy of the audit manifest to a custom location:

```bash
gh photos sync /backup/iphone gdrive:photos --save-audit-manifest /backups/audit/manifest-$(date +%Y%m%d).json
```

#### `--use-last-command`

Re-run the last successful command with the same parameters:

```bash
gh photos sync --use-last-command
# Loads settings from ~/gh-photos/manifest.json and re-executes
```

You can override specific parameters:

```bash
gh photos sync --use-last-command --dry-run
gh photos sync --use-last-command --remote s3:new-backup-location
```

### Example Workflows

#### 1. Initial Backup

```bash
gh photos sync /backup/iphone gdrive:photos --parallel 8 --checksum
```

Creates:

- `~/gh-photos/manifest.json`
- `~/gh-photos/manifest_2025-09-18T10-45-00Z.json`

#### 2. Repeat Last Backup

```bash
gh photos sync --use-last-command
```

Automatically uses the same:

- Backup path (`/backup/iphone`)
- Remote (`gdrive:photos`)
- All flags (`--parallel 8 --checksum`)

#### 3. Modify Previous Run

```bash
gh photos sync --use-last-command --include-hidden --dry-run
```

Uses previous settings but:

- Adds `--include-hidden`
- Enables `--dry-run` mode

#### 4. Archive Audit Trail

```bash
gh photos sync --use-last-command --save-audit-manifest /archive/audit-$(date +%Y%m%d).json
```

## Integration Examples

### Automated Backups

```bash
#!/bin/bash
# daily-backup.sh

# Use last successful configuration
gh photos sync --use-last-command --log-level info

# Archive the audit trail
cp ~/gh-photos/manifest.json /backup-logs/audit-$(date +%Y%m%d).json
```

### Monitoring

```bash
#!/bin/bash
# check-backup-status.sh

MANIFEST=~/gh-photos/manifest.json
if [[ -f "$MANIFEST" ]]; then
    FAILED=$(jq '.metadata.summary.assets_failed' "$MANIFEST")
    if [[ "$FAILED" -gt 0 ]]; then
        echo "Warning: $FAILED assets failed in last backup"
        jq -r '.assets[] | select(.status=="failed") | .local_path' "$MANIFEST"
    fi
fi
```

### Analytics

```bash
# Get total bytes transferred in last run
jq '.metadata.summary.bytes_transferred' ~/gh-photos/manifest.json

# List all uploaded files
jq -r '.assets[] | select(.status=="uploaded") | .remote_path' ~/gh-photos/manifest.json

# Get device information
jq '.metadata.device' ~/gh-photos/manifest.json
```

## Error Handling

- **No audit trail exists**: `--use-last-command` will show a clear error message
- **Corrupted manifest**: CLI will refuse to load and ask for manual backup path
- **Failed runs**: No manifest is created/updated (only successful runs are recorded)

## Future Enhancements

The audit trail system is designed for extensibility:

- Error tracking with codes and detailed messages
- Performance metrics (throughput, per-phase timing)
- Cloud provider metadata
- Compression/encryption details
- Schema versioning for backward compatibility

---

**Version**: 1.0.0  
**Implemented**: September 18, 2025
