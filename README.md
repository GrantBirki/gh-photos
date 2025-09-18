# gh-photos 📸

A gh cli extension for uploading iPhone photos from a backup dump to a remote target using rclone and logical defaults.

[![test](https://github.com/grantbirki/gh-photos/actions/workflows/test.yml/badge.svg)](https://github.com/grantbirki/gh-photos/actions/workflows/test.yml)
[![build](https://github.com/grantbirki/gh-photos/actions/workflows/build.yml/badge.svg)](https://github.com/grantbirki/gh-photos/actions/workflows/build.yml)
[![lint](https://github.com/grantbirki/gh-photos/actions/workflows/lint.yml/badge.svg)](https://github.com/grantbirki/gh-photos/actions/workflows/lint.yml)
[![release](https://github.com/grantbirki/gh-photos/actions/workflows/release.yml/badge.svg)](https://github.com/grantbirki/gh-photos/actions/workflows/release.yml)
![slsa-level3](docs/assets/slsa-level3.svg)

## About ⭐

This project is a [`gh cli`](https://github.com/cli/cli) extension that is used to TODO...

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

## Usage 🚀

TODO

### Command Line Options

| Flag | Description | Default |
|------|-------------|---------|

## Verifying Release Binaries 🔏

This project uses [goreleaser](https://goreleaser.com/) to build binaries and [actions/attest-build-provenance](https://github.com/actions/attest-build-provenance) to publish the provenance of the release.

You can verify the release binaries by following these steps:

1. Download a release from the [releases page](https://github.com/grantbirki/gh-photos/releases).
2. Verify it `gh attestation verify --owner grantbirki ~/Downloads/darwin-arm64` (an example for darwin-arm64).

---

Run `gh photos --help` for more information and full command/options usage.
