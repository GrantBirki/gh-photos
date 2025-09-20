# gh-photos 📸

[![test](https://github.com/grantbirki/gh-photos/actions/workflows/test.yml/badge.svg)](https://github.com/grantbirki/gh-photos/actions/workflows/test.yml)
[![build](https://github.com/grantbirki/gh-photos/actions/workflows/build.yml/badge.svg)](https://github.com/grantbirki/gh-photos/actions/workflows/build.yml)
[![lint](https://github.com/grantbirki/gh-photos/actions/workflows/lint.yml/badge.svg)](https://github.com/grantbirki/gh-photos/actions/workflows/lint.yml)
[![release](https://github.com/grantbirki/gh-photos/actions/workflows/release.yml/badge.svg)](https://github.com/grantbirki/gh-photos/actions/workflows/release.yml)
![slsa-level3](docs/assets/slsa-level3.svg)

A gh cli extension for uploading iPhone photos from a backup dump to a remote target using rclone and logical defaults.

![logo](docs/assets/logo.png)

## About ⭐

This project is a [`gh cli`](https://github.com/cli/cli) extension that extracts photos and videos from unencrypted iPhone backup directories and uploads them to cloud storage using [rclone](https://rclone.org/). The tool intelligently parses the `Photos.sqlite` database to classify assets by type and organizes them into a clean folder structure on your chosen cloud provider.

**Key Features:**

- 📱 Extracts photos from unencrypted iPhone backups
- 🗂️ Organizes uploads by date and asset type (`photos/YYYY/MM/DD/<category>/`)
- 🗂️ Flexible date-based folder depth (year, month, or day) with `--path-granularity` (`YYYY/`, `YYYY/MM/`, or `YYYY/MM/DD/`)
- 🔐 Privacy-safe defaults (excludes Hidden/Recently Deleted albums)
- ☁️ Supports all **rclone** remotes (Google Drive, S3, OneDrive, etc.)
- ⚡ Parallel uploads for faster performance
- Smart defaults (skips existing files to save bandwidth using rclone's native --ignore-existing)
- Optional remote pre-scan (`--remote-pre-scan`) to build an upfront skip plan (disabled by default for speed)
- 📋 Manifest generation for operation auditing
- 🧪 Dry-run mode for safe testing

## Installation 💻

Install this gh cli extension by running the following command:

```bash
gh extension install grantbirki/gh-photos
```

### Upgrading 📦

You can upgrade this extension by running the following command:

```bash
gh ext upgrade photos
```

## Finding iPhone Backup Paths 📱

The CLI automatically walks directory structures to find your iPhone backup. You can provide any of these common parent directories, and the tool will locate the actual backup folder:

> The folder might look something like this: `C:\Users\<username>\AppData\Roaming\Apple Computer\MobileSync\Backup\<device_uuid>`

### Windows Backup Locations

```bash
# Common Windows iPhone backup paths (the CLI will auto-navigate to the backup folder)
C:\Users\<username>\Apple\MobileSync\
C:\Users\<username>\Apple\MobileSync\Backup\
C:\Users\<username>\AppData\Roaming\Apple Computer\MobileSync\
C:\Users\<username>\AppData\Roaming\Apple Computer\MobileSync\Backup\
```

### macOS Backup Locations

```bash
# Common macOS iPhone backup paths (the CLI will auto-navigate to the backup folder)
~/Library/Application Support/MobileSync/
~/Library/Application Support/MobileSync/Backup/
/Users/<username>/Library/Application Support/MobileSync/
/Users/<username>/Library/Application Support/MobileSync/Backup/
```

**Smart Path Resolution**: The CLI automatically detects if you've provided a parent directory and will:

1. Look for a `Backup` subdirectory
2. Check if that contains `Manifest.db` or `Manifest.plist` files
3. If there's a single backup folder inside, navigate into it
4. Validate that it's a proper iPhone backup directory

This means you can simply point to `/Users/username/Library/Application Support/MobileSync/` and the tool will find the actual backup directory automatically.

## iTunes Backup Extraction 🗂️

The `extract` command allows you to extract unencrypted iTunes/Finder backups into a readable directory structure before processing. This is useful when your backup files are in the hashed format that iTunes uses internally.

### Why Extract?

iTunes and Finder create backups with hashed filenames (like `00/1a2b3c4e5f...`) instead of the original file names. The extract command:

- ✅ **Reconstructs original paths** using the backup's `Manifest.db`
- ✅ **Organizes by domain** (MediaDomain, HomeDomain, etc.)
- ✅ **Only supports unencrypted backups** (encrypted backups are rejected for security)
- ✅ **Shows progress and provides detailed summary**
- ✅ **Optionally verifies file integrity** with checksums

### Extract Examples

```bash
# Basic extraction (creates ./extracted-backup/)
gh photos extract /path/to/backup

# Extract to specific directory
gh photos extract /path/to/backup ./my-extracted-backup

# Extract with verification and progress
gh photos extract /backup ./extracted --verify --progress

# Skip files that already exist
gh photos extract /backup ./extracted --skip-existing
```

After extraction, you can run normal sync operations:

```bash
# Extract first, then sync using extracted directory
gh photos extract /path/to/backup ./extracted
gh photos sync ./extracted GoogleDriveRemote:photos
```

## Usage 🚀

The `gh-photos` extension provides three main commands for working with iPhone backup photos:

### Basic Commands

```bash
# Extract iTunes/Finder backup to readable directory structure
gh photos extract /path/to/backup
gh photos extract /path/to/backup ./extracted-backup

# Sync photos from iPhone backup to Google Drive
gh photos sync /path/to/backup GoogleDriveRemote:photos

# Dry run to preview what would be uploaded
gh photos sync /path/to/extracted/backup GoogleDriveRemote:photos --dry-run

# Validate an iPhone backup directory
gh photos validate /path/to/backup

# List assets found in backup
gh photos list /path/to/backup

# Upload to nested folder structure on the remote - This creates: Google Drive/Backups/iPhone/photos/YYYY/MM/DD/
gh photos sync /path/to/backup GoogleDriveRemote:Backups/iPhone/photos --ignore Thumbnails/*,derivatives/*

```

### Advanced Usage Examples

```bash
# Sync with custom settings (skips existing files by default)
gh photos sync /backup GoogleDriveRemote:photos \
  --include-hidden \
  --parallel 8 \
  --checksum \
  --save-manifest manifest.json

# Filter by date range and asset types
gh photos sync /backup s3:mybucket/photos \
  --start-date 2023-01-01 \
  --end-date 2023-12-31 \
  --types photos,videos

# Force overwrite existing files
gh photos sync /backup GoogleDriveRemote:photos \
  --force-overwrite \
  --verify

# Pre-scan the remote first (slower; only if you need an upfront skip plan)
gh photos sync /backup GoogleDriveRemote:photos \
  --remote-pre-scan \
  --skip-existing

# Organize by year/month only (reduces folder count)
gh photos sync /backup GoogleDriveRemote:photos --path-granularity month

# Organize by year only (flattest date structure)
gh photos sync /backup GoogleDriveRemote:photos --path-granularity year

# List assets with filtering
gh photos list /backup \
  --include-hidden \
  --types screenshots,burst \
  --format json
```

### The Commands that I use

> Hey @GrantBirki! Read this if you're looking to backup your phone again!

This section is mainly just notes for myself to backup my iPhone periodically.

1. Enter the iPhone unencrypted backup directory:

   ```cmd
   cd C:\Users\Birki\Apple\MobileSync\Backup\<device_uuid>
   ```

2. Extract the current iPhone backup directory to `D:\extracted`

   ```cmd
   gh photos extract . --output D:\extracted --progress --log-level debug
   ```

3. Sync the entire backup to Google Drive:

    ```cmd
    gh photos sync D:\extracted GoogleDriveRemote:Backups/iPhone/ --log-level debug --ignore "Thumbnails/*,derivatives/*" --path-granularity month
    ```

    > Sync the entire backup to Google Drive. This command is idempotent and can be run over and over again on the same extract directory if needed. For example, if you run it once and it fails half way through, you can simply restart it by running the same command again and it will skip over photos/videos that it has already uploaded (rclone does this by design) and pick back up with uploading the rest of the photos that it needs to.

In the case of your (my) `rclone` token being expired (likely given how in-frequently I do backups) you will need to run this command to get a new token if using Google Drive:

```bash
rclone config reconnect GoogleDriveRemote: # press "y" and open a browser to finish the re-auth flow through your google account
```

### Command Line Options

#### Global Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--no-color` | Disable colored output | `false` |
| `--verbose` | Enable verbose logging | `false` |
| `--log-level` | Set log level (debug, info, warn, error) | `info` |
| `--help` | Show help for command | - |
| `--version` | Show version information | - |

#### Sync Command Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--include-hidden` | Include assets flagged as hidden | `false` |
| `--include-recently-deleted` | Include assets flagged as recently deleted | `false` |
| `--dry-run` | Preview operations without uploading | `false` |
| `--skip-existing` | Skip files that already exist on remote (smart default) | `true` |
| `--remote-pre-scan` | Pre-scan remote to mark existing files before upload (slower; default is to skip during transfer) | `false` |
| `--force-overwrite` | Overwrite existing files on remote (opposite of --skip-existing) | `false` |
| `--verify` | Verify uploaded files match source | `false` |
| `--checksum` | Compute SHA256 checksums for assets | `false` |
| `--parallel` | Number of parallel uploads | `4` |
| `--save-manifest` | Path to save operation manifest (JSON) | - |
| `--types` | Asset types to include (photos,videos,screenshots,burst,live_photos) | all |
| `--start-date` | Start date filter (YYYY-MM-DD) | - |
| `--end-date` | End date filter (YYYY-MM-DD) | - |
| `--ignore` | Comma-separated glob patterns to ignore (e.g. `Thumbnails/*,derivatives/*`) | - |
| `--path-granularity` | Date folder depth: `year`, `month`, or `day` | `day` |

#### List Command Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--include-hidden` | Include hidden assets in listing | `false` |
| `--include-recently-deleted` | Include recently deleted assets in listing | `false` |
| `--types` | Filter by asset types | all |
| `--format` | Output format (table, json) | `table` |

#### Extract Command Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-o, --output` | Output directory for extracted files | `./extracted-backup` |
| `--skip-existing` | Skip files that already exist in output directory | `false` |
| `--verify` | Verify extracted files by comparing checksums (significantly slows extraction) | `false` |
| `--progress` | Show extraction progress during operation | `true` |

### Remote Existence & Skipping Strategy

By default, `gh-photos` does **not** enumerate the entire remote. It relies on rclone's native `--ignore-existing` behavior during transfer. This keeps startup fast and avoids potentially slow/fragile deep listings (e.g. on Google Drive).

You can choose between three modes:

| Mode | Flags | Behavior | Pros | Cons |
|------|-------|----------|------|------|
| Fast incremental (default) | `--skip-existing` (implicit) | No pre-listing; rclone skips as it copies | Fast start, minimal API calls | Upload plan can't pre-label skips |
| Pre-scan (explicit) | `--skip-existing --remote-pre-scan` | Lists remote paths to mark skips before uploading | Upfront insight (plan shows skip vs upload) | Slower start, more API requests |
| Force overwrite | `--force-overwrite` | Always uploads, overwriting existing | Ensures replacement | Extra bandwidth / potential remote versioning |

Recommendation: Only use `--remote-pre-scan` if you specifically need a detailed pre-upload plan. Otherwise stick with the default fast mode.

### Path Granularity (Date Folder Depth)

By default, assets are organized as: `YYYY/MM/DD/<type>/<filename>`.

You can reduce directory nesting with the `--path-granularity` flag:

| Value | Resulting Structure | Example Path |
|-------|---------------------|--------------|
| `day` (default) | `YYYY/MM/DD/<type>/` | `2024/03/18/photos/IMG_0001.HEIC` |
| `month` | `YYYY/MM/<type>/` | `2024/03/photos/IMG_0001.HEIC` |
| `year` | `YYYY/<type>/` | `2024/photos/IMG_0001.HEIC` |

Use `month` if you want only 12 folders per year per type, or `year` for the flattest structure while preserving type segregation.

### Environment Variables

`LOG_LEVEL` can be set to override the default logging level when `--log-level` isn't provided (e.g. `export LOG_LEVEL=debug`).

### Logging

Use `--log-level debug` (or `LOG_LEVEL=debug`) to see:

- rclone client initialization parameters
- Chunk sizing and batch grouping
- Remote pre-scan progress (when enabled)
- Batch upload start/end events and errors

This is especially helpful when diagnosing performance or remote auth issues.

## Command Metadata 📊

Both `sync` and `extract` commands automatically generate comprehensive metadata about the operation, including:

### **Metadata Includes:**

- ⏰ **UTC timestamp** of command completion (RFC3339 format)
- 💻 **System information**: OS, architecture, and version of the computer running the CLI
- 📱 **iOS backup details**: Device name, model, iOS version, backup date, and backup type
- 🖼️ **Asset type counts**: Photos, videos, Live Photos, screenshots, and burst photos detected

### **Metadata Output:**

- **Console Display**: Metadata summary is printed after successful operations
- **JSON Storage**: For `sync` commands, metadata is saved to the `--save-manifest` file
- **Extract Files**: For `extract` commands, metadata is saved to `extraction-metadata.json` in the output directory

### **Example Metadata Output:**

```text
📊 Command Metadata Summary:
  Completed at: 2024-01-15T10:30:45Z
  CLI version: v0.0.8-dev
  System: darwin arm64 (macOS 15.1.1)

📱 iOS Backup Info:
  Backup path: /Users/username/Library/Application Support/MobileSync/Backup/12345...
  Backup type: hashed
  Encrypted: false
  Device name: John's iPhone
  Device model: iPhone14,2
  iOS version: 17.6
  Backup date: 2024-01-14T22:15:00Z
  Total files: 50670

🖼️  Asset Type Counts:
  Photos: 2847
  Videos: 156
  Live Photos: 23
  Screenshots: 89
  Burst: 12
  Total: 3127
```

This metadata helps with:

- **Operation auditing** and record-keeping
- **System compatibility** verification
- **Backup source** identification and validation
- **Asset inventory** tracking across different operations

## Verifying Release Binaries 🔏

This project uses [goreleaser](https://goreleaser.com/) to build binaries and [actions/attest-build-provenance](https://github.com/actions/attest-build-provenance) to publish the provenance of the release.

You can verify the release binaries by following these steps:

1. Download a release from the [releases page](https://github.com/grantbirki/gh-photos/releases).
2. Verify it `gh attestation verify --owner grantbirki ~/Downloads/darwin-arm64` (an example for darwin-arm64).

---

Run `gh photos --help` for more information and full command/options usage.
