package rclone

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/grantbirki/gh-photos/internal/logger"
	"github.com/grantbirki/gh-photos/internal/manifest"
)

// Client wraps rclone operations
type Client struct {
	remote              string
	parallel            int
	verify              bool
	dryRun              bool
	skipExisting        bool
	remotePreScan       bool
	logger              *logger.Logger
	logLevel            string
	backupPath          string        // Added to support metadata file discovery
	batchTimeout        time.Duration // Timeout for individual batch operations
	startupTestComplete bool          // Track if startup connectivity test has been done

	// Cached link capability detection to avoid repeated failing syscalls on Windows
	symlinkCapChecked bool
	symlinkSupported  bool
	hardlinkChecked   bool
	hardlinkSupported bool
}

// buildRemotePath safely constructs a remote destination path ensuring only one colon
// between remote name and path, normalizing slashes and removing duplicate separators.
func (c *Client) buildRemotePath(subPath string) string {
	spec := c.remote
	remoteName := spec
	basePath := ""
	if idx := strings.Index(spec, ":"); idx != -1 { // remote contains embedded path
		remoteName = spec[:idx]
		basePath = spec[idx+1:]
	}

	hadLeadingSlash := strings.HasPrefix(basePath, "/")

	// Helper function to normalize path separators and clean up duplicates
	normalizePath := func(p string, preserveLeadingSlash bool) string {
		// Convert backslashes to forward slashes
		p = strings.ReplaceAll(p, "\\", "/")

		// Collapse duplicate slashes
		for strings.Contains(p, "//") {
			p = strings.ReplaceAll(p, "//", "/")
		}

		// Handle leading/trailing slashes based on context
		if preserveLeadingSlash && strings.HasPrefix(p, "/") {
			// Trim trailing slash (except for root)
			if len(p) > 1 {
				p = strings.TrimSuffix(p, "/")
			}
		} else {
			// Trim leading and trailing slashes for relative paths
			p = strings.Trim(p, "/")
		}

		return p
	}

	basePath = normalizePath(basePath, true) // Preserve leading slash for absolute paths
	subPath = normalizePath(subPath, false)  // Always relative, no leading slash

	// Restore leading slash if original spec indicated absolute path (e.g., sftp:/absolute/path)
	if hadLeadingSlash && basePath != "" && !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}

	var joined string
	switch {
	case basePath == "" && subPath == "":
		joined = ""
	case basePath != "" && subPath == "":
		joined = basePath
	case basePath == "" && subPath != "":
		joined = subPath
	default:
		// If base is absolute (starts with /), don't double the slash when joining
		if strings.HasPrefix(basePath, "/") {
			joined = basePath + "/" + subPath
		} else {
			joined = basePath + "/" + subPath
		}
	}

	result := remoteName + ":" + joined

	// Add debug logging for path construction (only if debug logging is enabled)
	if c != nil && c.logLevel == "debug" {
		c.logDebug("buildRemotePath path construction",
			"input_subPath", subPath,
			"parsed_remoteName", remoteName,
			"parsed_basePath", basePath,
			"normalized_basePath", basePath,
			"normalized_subPath", subPath,
			"joined_path", joined,
			"final_result", result,
		)
	}

	return result
}

// createFileLink creates a symlink with Windows fallback support
// On Windows, if symlink creation fails (due to permissions), it falls back to hard links
// and if that fails, copies the file content
func (c *Client) createFileLink(sourcePath, targetPath string) error {
	// Detect symlink capability once
	if !c.symlinkCapChecked {
		c.symlinkCapChecked = true
		testSrc := sourcePath
		// create a tiny temp file if needed for capability test
		if _, err := os.Stat(testSrc); err != nil {
			tmp, tmpErr := os.CreateTemp("", "gh-photos-linktest-*")
			if tmpErr == nil {
				tmp.WriteString("test")
				tmp.Close()
				testSrc = tmp.Name()
				defer os.Remove(testSrc)
			}
		}
		tmpDst, _ := os.CreateTemp("", "gh-photos-linktest-dst-*")
		tmpDst.Close()
		os.Remove(tmpDst.Name())
		if err := os.Symlink(testSrc, tmpDst.Name()); err == nil {
			c.symlinkSupported = true
			os.Remove(tmpDst.Name())
			c.logDebug("symlink capability detected - will use symlinks for staging")
		} else {
			c.symlinkSupported = false
			c.logDebug("symlink capability not available - will skip symlink attempts", "error", err)
		}
	}

	if c.symlinkSupported {
		if err := os.Symlink(sourcePath, targetPath); err == nil {
			return nil
		} else {
			// If symlink unexpectedly fails repeatedly, disable for remainder
			c.symlinkSupported = false
			c.logDebug("disabling symlink usage after failure", "error", err)
		}
	}

	// Detect hard link capability once (generally supported on Windows NTFS & Unix same volume)
	if !c.hardlinkChecked {
		c.hardlinkChecked = true
		testSrc := sourcePath
		if _, err := os.Stat(testSrc); err != nil {
			tmp, tmpErr := os.CreateTemp("", "gh-photos-hardlinktest-*")
			if tmpErr == nil {
				tmp.WriteString("test")
				tmp.Close()
				testSrc = tmp.Name()
				defer os.Remove(testSrc)
			}
		}
		tmpDst, _ := os.CreateTemp("", "gh-photos-hardlinktest-dst-*")
		tmpDst.Close()
		os.Remove(tmpDst.Name())
		if err := os.Link(testSrc, tmpDst.Name()); err == nil {
			c.hardlinkSupported = true
			os.Remove(tmpDst.Name())
			c.logDebug("hard link capability detected - will use hard links for staging")
		} else {
			c.hardlinkSupported = false
			c.logDebug("hard link capability not available - will copy files for staging", "error", err)
		}
	}

	if c.hardlinkSupported {
		if err := os.Link(sourcePath, targetPath); err == nil {
			return nil
		} else {
			c.logDebug("hard link failed, falling back to copy", "error", err)
		}
	}

	// Final fallback: copy
	return c.copyFile(sourcePath, targetPath)
}

// copyFile copies file content from source to target
func (c *Client) copyFile(sourcePath, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create target file: %w", err)
	}
	defer targetFile.Close()

	_, err = targetFile.ReadFrom(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	c.logDebug("file copied successfully", "source", sourcePath, "target", targetPath)
	return nil
}

// Helper logging methods to avoid nil checks everywhere
func (c *Client) logDebug(msg string, args ...any) {
	if c.logger != nil {
		c.logger.Debug(msg, args...)
	}
}

func (c *Client) logInfo(msg string, args ...any) {
	if c.logger != nil {
		c.logger.Info(msg, args...)
	}
}

func (c *Client) logWarn(msg string, args ...any) {
	if c.logger != nil {
		c.logger.Warn(msg, args...)
	}
}

func (c *Client) logError(msg string, args ...any) {
	if c.logger != nil {
		c.logger.Error(msg, args...)
	}
}

// setupRcloneCmd configures an rclone command with proper environment
func setupRcloneCmd(cmd *exec.Cmd) {
	// Inherit the parent process's environment to ensure rclone has access to config
	cmd.Env = os.Environ()

	// Set working directory to user home to help rclone find config files
	if homeDir, err := os.UserHomeDir(); err == nil {
		cmd.Dir = homeDir
	}
}

// CreateClient creates a new rclone client
func CreateClient(remote string, parallel int, verify, dryRun, skipExisting bool, logger *logger.Logger, logLevel string) *Client {
	return &Client{
		remote:        remote,
		parallel:      parallel,
		verify:        verify,
		dryRun:        dryRun,
		skipExisting:  skipExisting,
		remotePreScan: false, // Disabled for now
		logger:        logger,
		logLevel:      logLevel,
		batchTimeout:  30 * time.Minute, // Default 30 minute timeout per batch
	}
}

// SetBackupPath sets the backup path for metadata file discovery
func (c *Client) SetBackupPath(backupPath string) {
	c.backupPath = backupPath
}

// SetBatchTimeout sets the timeout for individual batch operations
func (c *Client) SetBatchTimeout(timeout time.Duration) {
	c.batchTimeout = timeout
}

// getBaseRemote extracts the base remote name from the remote specification
func (c *Client) getBaseRemote() string {
	if strings.Contains(c.remote, ":") && !strings.HasSuffix(c.remote, ":") {
		return c.remote[:strings.Index(c.remote, ":")+1]
	}
	if !strings.HasSuffix(c.remote, ":") {
		return c.remote + ":"
	}
	return c.remote
}

// isGoogleDriveRemote checks if the remote is a Google Drive remote
func (c *Client) isGoogleDriveRemote() bool {
	// Extract the remote name (everything before the first colon)
	remoteName := c.remote
	if idx := strings.Index(c.remote, ":"); idx != -1 {
		remoteName = c.remote[:idx]
	}

	// Check if it's a Google Drive remote by examining the remote name
	// Common patterns: gdrive, gdrive-backup, my-gdrive, etc.
	remoteLower := strings.ToLower(remoteName)
	return strings.Contains(remoteLower, "gdrive") ||
		strings.Contains(remoteLower, "google") ||
		strings.Contains(remoteLower, "drive")
}

// handleDryRunBatch processes batch upload in dry-run mode
func (c *Client) handleDryRunBatch(entries []manifest.Entry, updateCallback func(int, manifest.OperationStatus, string), progressCallback ProgressCallback) error {
	for i, entry := range entries {
		if progressCallback != nil {
			progressCallback(i, len(entries), filepath.Base(entry.SourcePath))
		}
		fmt.Printf("[DRY-RUN] Would upload: %s -> %s:%s\n",
			entry.SourcePath, c.remote, entry.TargetPath)
		updateCallback(i, manifest.StatusUploaded, "")
	}
	if progressCallback != nil {
		progressCallback(len(entries), len(entries), "")
	}
	c.logInfo("dry-run batch upload complete")
	return nil
}

// performRemotePreScan checks which files already exist on the remote
func (c *Client) performRemotePreScan(ctx context.Context, entries []manifest.Entry) map[string]bool {
	c.logInfo("Pre-scanning remote for existing files (may be slower)", "files_to_check", len(entries))

	existingFiles, err := c.BatchCheckRemoteExists(ctx, entries)
	if err != nil {
		c.logWarn("Failed to batch check remote files, falling back to individual checks", "error", err.Error())
		return nil
	}

	c.logInfo("Remote file check complete", "existing_files", len(existingFiles))
	return existingFiles
}

// RunStartupConnectivityTest runs comprehensive connectivity tests at startup
func (c *Client) RunStartupConnectivityTest() error {
	if c.startupTestComplete {
		c.logDebug("startup connectivity test already completed, skipping")
		return nil
	}

	c.logInfo("running startup connectivity tests...")

	// Test 1: Basic remote connectivity
	baseRemote := c.getBaseRemote()
	c.logDebug("testing base remote connectivity", "base_remote", baseRemote)
	if err := c.testRemoteConnectivity(baseRemote); err != nil {
		return fmt.Errorf("base remote connectivity test failed: %w", err)
	}

	// Test 2: Upload extraction metadata to verify write capability
	if err := c.testRemoteWriteCapabilityStartup(); err != nil {
		c.logWarn("extraction metadata upload test failed", "error", err)
		// Don't fail startup on metadata upload failure - it's just a diagnostic
	}

	c.startupTestComplete = true
	c.logInfo("startup connectivity tests completed successfully")
	return nil
}

// findExtractionMetadataFile looks for extraction-metadata.json in the backup path
func (c *Client) findExtractionMetadataFile() string {
	if c.backupPath == "" {
		return ""
	}

	metadataPath := filepath.Join(c.backupPath, "extraction-metadata.json")
	if _, err := os.Stat(metadataPath); err == nil {
		return metadataPath
	}

	// If not found, return empty string
	return ""
}

// getTimestampFromMetadata reads the completed_at timestamp from extraction metadata file
func (c *Client) getTimestampFromMetadata(metadataPath string) (string, error) {
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return "", fmt.Errorf("failed to read metadata file: %w", err)
	}

	// Parse JSON to extract command_metadata.completed_at
	var metadata struct {
		CommandMetadata struct {
			CompletedAt string `json:"completed_at"`
		} `json:"command_metadata"`
	}

	if err := json.Unmarshal(data, &metadata); err != nil {
		return "", fmt.Errorf("failed to parse metadata JSON: %w", err)
	}

	if metadata.CommandMetadata.CompletedAt == "" {
		return "", fmt.Errorf("completed_at not found in metadata")
	}

	// Parse the RFC3339 timestamp and convert to our desired format
	parsedTime, err := time.Parse(time.RFC3339, metadata.CommandMetadata.CompletedAt)
	if err != nil {
		return "", fmt.Errorf("failed to parse completed_at timestamp: %w", err)
	}

	// Convert to our filename format
	return parsedTime.Format("2006-01-02T15-04-05Z"), nil
}

// UploadEntry uploads a single manifest entry
func (c *Client) UploadEntry(ctx context.Context, entry manifest.Entry) error {
	if c.dryRun {
		fmt.Printf("[DRY-RUN] Would upload: %s -> %s:%s\n",
			entry.SourcePath, c.remote, entry.TargetPath)
		return nil
	}

	// Build rclone command
	args := []string{"copyto"}

	if c.skipExisting {
		args = append(args, "--ignore-existing")
	}

	if c.verify {
		args = append(args, "--check-first")
	}

	// Add source and destination
	args = append(args, entry.SourcePath)
	// Normalize to forward slashes for remote destinations; rclone tolerates but we keep consistent
	normalized := strings.ReplaceAll(entry.TargetPath, "\\", "/")
	dest := c.buildRemotePath(normalized)
	args = append(args, dest)

	c.logDebug("starting single file upload",
		"source", entry.SourcePath,
		"target", entry.TargetPath,
		"remote", c.remote,
		"args", strings.Join(args, " "))

	// Execute rclone command
	cmd := exec.CommandContext(ctx, "rclone", args...)
	setupRcloneCmd(cmd)

	// Capture stderr to avoid direct terminal output after cancellation
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		// Log stderr output for debugging
		if stderrOutput := stderrBuf.String(); stderrOutput != "" {
			c.logError("rclone stderr output", "stderr", stderrOutput)
		}

		// Check if error was due to context cancellation
		if ctx.Err() != nil {
			c.logDebug("single upload cancelled", "source", entry.SourcePath, "context_err", ctx.Err())
			return ctx.Err()
		}

		c.logError("single upload failed", "error", err, "source", entry.SourcePath, "target", entry.TargetPath)
		return fmt.Errorf("rclone upload failed: %w", err)
	}

	c.logDebug("single file upload complete", "source", entry.SourcePath, "target", entry.TargetPath)
	return nil
}

// ProgressCallback provides upload progress updates
type ProgressCallback func(completed, total int, currentFile string)

// UploadBatch uploads multiple entries using efficient batch operations with progress reporting
// This single method handles all upload operations regardless of dataset size for optimal performance
func (c *Client) UploadBatch(ctx context.Context, entries []manifest.Entry, updateCallback func(int, manifest.OperationStatus, string), progressCallback ProgressCallback) error {
	if len(entries) == 0 {
		c.logInfo("no entries provided to UploadBatch")
		return nil
	}
	c.logInfo("starting batch upload", "entries", len(entries))

	// Handle dry run mode
	if c.dryRun {
		return c.handleDryRunBatch(entries, updateCallback, progressCallback)
	}

	// Determine optimal chunk size based on dataset size for better progress feedback
	chunkSize := 200 // Default chunk size for good progress reporting
	if len(entries) <= 50 {
		chunkSize = len(entries) // Single chunk for small datasets
	} else if len(entries) <= 500 {
		chunkSize = 100 // Medium chunks for moderate datasets
	}

	c.logDebug("calculated chunk size", "chunk_size", chunkSize)
	// Process entries in chunks
	for i := 0; i < len(entries); i += chunkSize {
		end := i + chunkSize
		if end > len(entries) {
			end = len(entries)
		}

		chunk := entries[i:end]
		c.logDebug("processing chunk", "start_index", i, "end_index", end, "chunk_len", len(chunk))
		if err := c.uploadChunk(ctx, chunk, entries, i, updateCallback, progressCallback); err != nil {
			return err
		}
	}

	c.logInfo("batch upload complete")
	return nil
}

// uploadChunk handles uploading a chunk of entries efficiently using rclone batch operations
func (c *Client) uploadChunk(ctx context.Context, chunk []manifest.Entry, allEntries []manifest.Entry, baseIndex int, updateCallback func(int, manifest.OperationStatus, string), progressCallback ProgressCallback) error {
	// Group entries by target directory to optimize rclone calls
	dirGroups := make(map[string][]manifest.Entry)
	for _, entry := range chunk {
		dir := filepath.Dir(entry.TargetPath)
		if dir == "." {
			dir = ""
		}
		dirGroups[dir] = append(dirGroups[dir], entry)
	}
	c.logDebug("upload chunk grouping complete", "groups", len(dirGroups), "base_index", baseIndex)

	completed := baseIndex
	total := len(allEntries)

	// Process each directory group
	for targetDir, groupEntries := range dirGroups {
		// Early abort if context cancelled before processing this group
		if ctx.Err() != nil {
			c.logDebug("context cancelled before staging directory group", "dir", targetDir)
			return ctx.Err()
		}
		// Normalize target directory path for remote (forward slashes)
		normalizedDir := strings.ReplaceAll(targetDir, "\\", "/")
		if normalizedDir == "." || normalizedDir == "" {
			normalizedDir = "" // root of remote path
		}
		c.logDebug("processing directory group", "dir", targetDir, "files", len(groupEntries))
		// Create temporary directory for this batch
		tempDir, err := os.MkdirTemp("", "gh-photos-batch-*")
		if err != nil {
			c.logError("temp directory creation failed", "error", err)
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
		defer os.RemoveAll(tempDir)

		// Create directory structure and copy files to temp location
		filePrepCount := 0
		for _, entry := range groupEntries {
			if ctx.Err() != nil {
				c.logDebug("context cancelled during staging loop", "dir", targetDir)
				return ctx.Err()
			}
			if progressCallback != nil {
				progressCallback(completed, total, fmt.Sprintf("Preparing %s", filepath.Base(entry.SourcePath)))
			}

			// Normalize target path to use forward slashes for consistent cross-platform behavior
			normalizedTargetPath := strings.ReplaceAll(entry.TargetPath, "\\", "/")

			// Create target directory structure in temp using normalized path
			tempTargetPath := filepath.Join(tempDir, normalizedTargetPath)
			if err := os.MkdirAll(filepath.Dir(tempTargetPath), 0755); err != nil {
				return fmt.Errorf("failed to create temp directory structure: %w", err)
			}

			// Create symlink to avoid copying large files, with Windows fallback
			if err := c.createFileLink(entry.SourcePath, tempTargetPath); err != nil {
				c.logError("failed to create file link", "error", err, "source", entry.SourcePath, "target", tempTargetPath)
				return fmt.Errorf("failed to create file link: %w", err)
			}
			filePrepCount++
			if filePrepCount%250 == 0 && c.logLevel == "debug" { // periodic staging progress
				c.logDebug("staging progress", "prepared", filePrepCount, "group_total", len(groupEntries), "dir", targetDir)
			}
		}

		// Build rclone command for batch upload
		// Copy from tempDir to remote root - rclone will recreate the directory structure
		dest := c.buildRemotePath("")
		c.logDebug("batch upload destination", "dest", dest, "temp_dir", tempDir)

		args := []string{"copy", tempDir, dest}

		if c.skipExisting {
			args = append(args, "--ignore-existing")
		}

		if c.verify {
			args = append(args, "--check-first")
		}

		// Add symlink following for Windows compatibility
		args = append(args, "--copy-links")

		// Add parallelization for faster uploads
		if c.parallel > 1 {
			args = append(args, fmt.Sprintf("--transfers=%d", c.parallel))
		}

		// Add Google Drive specific optimizations
		if c.isGoogleDriveRemote() {
			args = append(args, "--fast-list")                // 20x faster directory listing
			args = append(args, "--drive-chunk-size=256M")    // Larger chunks for big files
			args = append(args, "--drive-upload-cutoff=256M") // When to use resumable uploads
			args = append(args, "--tpslimit=10")              // Respect API rate limits
		}

		// Add progress reporting
		args = append(args, "--progress", "--stats-one-line")

		// Add verbose output in debug mode
		if c.logLevel == "debug" {
			args = append(args, "--verbose")
		}

		c.logDebug("executing rclone batch", "dir", targetDir, "file_count", len(groupEntries), "args", strings.Join(args, " "))

		// Create a timeout context for this batch operation
		batchCtx, batchCancel := context.WithTimeout(ctx, c.batchTimeout)
		defer batchCancel()

		cmd := exec.CommandContext(batchCtx, "rclone", args...)
		setupRcloneCmd(cmd)

		// Capture stderr to avoid direct terminal output after cancellation
		var stderrBuf bytes.Buffer
		cmd.Stderr = &stderrBuf

		// Capture output for progress parsing
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("failed to create stdout pipe: %w", err)
		}

		if err := cmd.Start(); err != nil {
			c.logError("failed to start rclone batch", "error", err, "dir", targetDir)
			return fmt.Errorf("failed to start rclone: %w", err)
		}

		// Parse progress output with context cancellation support
		scanner := bufio.NewScanner(stdout)
		done := make(chan bool)
		scanErr := make(chan error, 1)

		go func() {
			defer close(done)
			for scanner.Scan() {
				select {
				case <-batchCtx.Done():
					// Context cancelled, stop reading
					scanErr <- batchCtx.Err()
					return
				default:
					line := scanner.Text()
					c.logDebug("rclone output", "line", line)
					if progressCallback != nil && strings.Contains(line, "Transferred:") {
						progressCallback(completed, total, fmt.Sprintf("Uploading batch (%d files)", len(groupEntries)))
					}
				}
			}
			if err := scanner.Err(); err != nil {
				scanErr <- err
			}
		}()

		// Wait for either scanning to complete or context cancellation
		select {
		case <-done:
			// Scanning completed normally
		case err := <-scanErr:
			// Error during scanning
			c.logDebug("error during stdout scanning", "error", err, "dir", targetDir)
		case <-batchCtx.Done():
			// Context cancelled - rclone should terminate due to CommandContext
			if batchCtx.Err() == context.DeadlineExceeded {
				c.logError("rclone batch timed out", "dir", targetDir, "timeout", c.batchTimeout)
			} else {
				c.logDebug("upload cancelled by context", "dir", targetDir)
			}
		}

		if err := cmd.Wait(); err != nil {
			// Log stderr output for debugging
			if stderrOutput := stderrBuf.String(); stderrOutput != "" {
				c.logError("rclone stderr output", "stderr", stderrOutput)
			}

			// Check if error was due to context cancellation or timeout
			if batchCtx.Err() != nil {
				if batchCtx.Err() == context.DeadlineExceeded {
					c.logError("rclone batch failed due to timeout", "dir", targetDir, "timeout", c.batchTimeout)
					return fmt.Errorf("batch upload timed out after %v", c.batchTimeout)
				}
				c.logDebug("rclone batch cancelled", "dir", targetDir, "context_err", batchCtx.Err())
				return batchCtx.Err()
			}

			c.logError("rclone batch failed", "error", err, "dir", targetDir, "files", len(groupEntries), "exit_code", cmd.ProcessState.ExitCode())
			// Mark all entries in this batch as failed
			for _, entry := range groupEntries {
				// Find original index
				originalIndex := -1
				for j, originalEntry := range allEntries {
					if originalEntry.SourcePath == entry.SourcePath {
						originalIndex = j
						break
					}
				}
				if originalIndex >= 0 {
					updateCallback(originalIndex, manifest.StatusFailed, fmt.Sprintf("batch upload failed: %v", err))
				}
			}
			return fmt.Errorf("rclone batch upload failed: %w", err)
		}

		// Always log stderr output even on success to catch warnings
		if stderrOutput := stderrBuf.String(); stderrOutput != "" {
			c.logDebug("rclone stderr output (success case)", "stderr", stderrOutput)
		}

		c.logDebug("rclone batch complete", "dir", targetDir, "files", len(groupEntries), "exit_code", cmd.ProcessState.ExitCode())

		// Mark all entries in this batch as successful
		for _, entry := range groupEntries {
			completed++
			// Find original index
			originalIndex := -1
			for j, originalEntry := range allEntries {
				if originalEntry.SourcePath == entry.SourcePath {
					originalIndex = j
					break
				}
			}
			if originalIndex >= 0 {
				updateCallback(originalIndex, manifest.StatusUploaded, "")
			}

			if progressCallback != nil {
				progressCallback(completed, total, filepath.Base(entry.SourcePath))
			}
		}
	}

	return nil
}

// CheckRemoteExists checks if a file exists on the remote
func (c *Client) CheckRemoteExists(ctx context.Context, remotePath string) (bool, error) {
	fullPath := fmt.Sprintf("%s:%s", c.remote, remotePath)

	args := []string{"lsf", fullPath}

	// Add Google Drive optimizations for single file check
	if c.isGoogleDriveRemote() {
		args = append(args, "--fast-list")
	}

	cmd := exec.CommandContext(ctx, "rclone", args...)
	setupRcloneCmd(cmd)
	output, err := cmd.Output()

	if err != nil {
		// If the command fails, the file doesn't exist
		c.logDebug("remote file check - not found or error", "path", fullPath, "error", err)
		return false, nil
	}

	exists := strings.TrimSpace(string(output)) != ""
	c.logDebug("remote file check result", "path", fullPath, "exists", exists)
	return exists, nil
}

// BatchCheckRemoteExists efficiently checks which files exist on remote by listing all files once
func (c *Client) BatchCheckRemoteExists(ctx context.Context, entries []manifest.Entry) (map[string]bool, error) {
	// Extract unique directories to minimize the scope of our listing
	dirs := make(map[string]bool)
	for _, entry := range entries {
		dir := filepath.Dir(entry.TargetPath)
		if dir != "." && dir != "/" {
			dirs[dir] = true
		}
	}

	existingFiles := make(map[string]bool)

	// If we have many different directories, just list everything recursively
	if len(dirs) > 50 {
		c.logDebug("many directories - listing entire remote recursively", "dir_count", len(dirs))
		return c.listAllRemoteFiles(ctx)
	}

	// Otherwise, list each directory separately to be more efficient
	for dir := range dirs {
		if ctx.Err() != nil {
			c.logDebug("context cancelled before directory listing", "dir", dir)
			return existingFiles, ctx.Err()
		}
		remotePath := fmt.Sprintf("%s:%s", c.remote, dir)
		args := []string{"lsf", remotePath, "-R"}

		// Add Google Drive optimizations for directory listing
		if c.isGoogleDriveRemote() {
			args = append(args, "--fast-list")
		}

		cmd := exec.CommandContext(ctx, "rclone", args...)
		setupRcloneCmd(cmd)
		output, err := cmd.Output()

		if err != nil {
			// Directory might not exist, continue with others
			c.logDebug("directory listing failed or does not exist", "dir", dir, "error", err)
			continue
		}

		files := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, file := range files {
			if file != "" {
				// Construct full path relative to target
				fullPath := filepath.Join(dir, file)
				existingFiles[fullPath] = true
			}
		}
		c.logDebug("directory listing processed", "dir", dir, "files", len(files))
	}

	c.logDebug("batch remote existence check complete", "files_found", len(existingFiles))
	return existingFiles, nil
}

// listAllRemoteFiles lists all files on the remote recursively
func (c *Client) listAllRemoteFiles(ctx context.Context) (map[string]bool, error) {
	remotePath := fmt.Sprintf("%s:", c.remote)
	args := []string{"lsf", remotePath, "-R"}

	// Add Google Drive optimizations for recursive listing
	if c.isGoogleDriveRemote() {
		args = append(args, "--fast-list")
	}

	cmd := exec.CommandContext(ctx, "rclone", args...)
	setupRcloneCmd(cmd)
	output, err := cmd.Output()

	if err != nil {
		c.logError("failed to list all remote files", "error", err)
		return nil, fmt.Errorf("failed to list remote files: %w", err)
	}

	existingFiles := make(map[string]bool)
	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, file := range files {
		if ctx.Err() != nil {
			c.logDebug("context cancelled during remote file accumulation", "processed", len(existingFiles))
			return existingFiles, ctx.Err()
		}
		if file != "" {
			existingFiles[file] = true
		}
	}
	c.logDebug("listed all remote files", "count", len(existingFiles))

	return existingFiles, nil
}

// VerifyUpload verifies that an uploaded file matches the source
func (c *Client) VerifyUpload(ctx context.Context, entry manifest.Entry) error {
	if c.dryRun {
		fmt.Printf("[DRY-RUN] Would verify: %s:%s\n", c.remote, entry.TargetPath)
		return nil
	}

	fullPath := fmt.Sprintf("%s:%s", c.remote, entry.TargetPath)

	// Use rclone check to verify the file
	cmd := exec.CommandContext(ctx, "rclone", "check", entry.SourcePath, fullPath)
	setupRcloneCmd(cmd)

	c.logDebug("verifying upload", "source", entry.SourcePath, "target", fullPath)
	if err := cmd.Run(); err != nil {
		c.logError("verification failed", "error", err, "source", entry.SourcePath, "target", fullPath)
		return fmt.Errorf("verification failed: %w", err)
	}

	c.logDebug("verification successful", "target", fullPath)
	return nil
}

// ValidateRcloneInstallation checks if rclone is available and properly configured
func ValidateRcloneInstallation(log *logger.Logger) error {
	// Check if rclone is in PATH
	_, err := exec.LookPath("rclone")
	if err != nil {
		return fmt.Errorf("rclone not found in PATH: %w", err)
	}

	// Check rclone version
	cmd := exec.Command("rclone", "version")
	setupRcloneCmd(cmd)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get rclone version: %w", err)
	}

	version := string(output)
	if !strings.Contains(version, "rclone") {
		return fmt.Errorf("unexpected rclone version output: %s", version)
	}
	if log != nil {
		log.Debug("validated rclone installation", "version_output", strings.TrimSpace(version))
	}

	return nil
}

// ValidateRemote checks if the specified remote is configured
func ValidateRemote(remote string, log *logger.Logger) error {
	cmd := exec.Command("rclone", "listremotes")
	setupRcloneCmd(cmd)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list rclone remotes: %w", err)
	}

	remotes := strings.Split(string(output), "\n")
	remoteName := strings.Split(remote, ":")[0] + ":"

	for _, r := range remotes {
		if strings.TrimSpace(r) == remoteName {
			if log != nil {
				log.Debug("remote found", "remote", remoteName)
			}
			return nil
		}
	}

	return fmt.Errorf("remote '%s' not found in configured remotes", remoteName)
}

// ValidateRemoteAuthentication tests if the remote is accessible and authenticated
func ValidateRemoteAuthentication(remote string, log *logger.Logger) error {
	// Extract remote name from full remote path (e.g., "GoogleDriveRemote:path/to/dir" -> "GoogleDriveRemote")
	remoteName := remote
	if colonIndex := strings.Index(remote, ":"); colonIndex != -1 {
		remoteName = remote[:colonIndex]
	}

	// First check if rclone is available in PATH
	rclonePath, err := exec.LookPath("rclone")
	if err != nil {
		return fmt.Errorf("rclone binary not found in PATH: %w", err)
	}

	// Test authentication by trying to list the remote root directory
	cmd := exec.Command("rclone", "lsf", remoteName+":", "--max-depth", "1")
	setupRcloneCmd(cmd)

	// Capture both stdout and stderr to get better error information
	output, err := cmd.CombinedOutput()
	if err != nil {
		if log != nil {
			log.Error("remote authentication failed", "remote", remoteName, "error", err, "output", string(output))
		}
		return fmt.Errorf("remote authentication failed - rclone path: %s, error: %v, output: %s", rclonePath, err, string(output))
	}
	if log != nil {
		log.Debug("remote authentication successful", "remote", remoteName)
	}

	return nil
}

// CreateUploadPlan creates a plan for uploading assets
func (c *Client) CreateUploadPlan(ctx context.Context, entries []manifest.Entry) ([]UploadPlanEntry, error) {
	// Special handling for the metadata file: always upload if not existing, don't skip
	metadataPath := c.findExtractionMetadataFile()
	if metadataPath != "" {
		c.logDebug("found metadata file", "path", metadataPath)

		// Get timestamp for metadata file naming
		timestamp, err := c.getTimestampFromMetadata(metadataPath)
		if err != nil {
			c.logWarn("failed to get timestamp from metadata file", "error", err)
			timestamp = time.Now().UTC().Format("2006-01-02T15-04-05Z")
		}

		// Define remote path for metadata file
		remoteMetadataPath := c.buildRemotePath("metadata/extraction-metadata-" + timestamp + ".json")

		// Since remotePreScan is disabled, we cannot check for existing files here.
		// We will rely on rclone's --ignore-existing flag during the copy operation.
		c.logDebug("uploading metadata file to remote", "local_path", metadataPath, "remote_path", remoteMetadataPath)
		cmd := exec.CommandContext(ctx, "rclone", "copyto", "--ignore-existing", metadataPath, remoteMetadataPath)
		setupRcloneCmd(cmd)
		output, err := cmd.CombinedOutput()
		if err != nil {
			c.logError("failed to upload metadata file", "error", err, "output", string(output))
			// We can continue without the metadata file, so we just log the error.
		} else {
			c.logDebug("successfully uploaded metadata file")
		}
	}

	// Create the upload plan
	var planEntries []UploadPlanEntry

	// Populate the upload plan
	for _, entry := range entries {
		planEntry := UploadPlanEntry{
			Entry:  entry,
			Action: ActionUpload,
		}

		// Since remotePreScan is disabled, we assume all files need to be uploaded.
		// rclone's --ignore-existing will handle skipping at runtime.
		planEntries = append(planEntries, planEntry)
	}

	return planEntries, nil
}

// UploadAction represents the action to take for an upload
type UploadAction string

const (
	ActionUpload UploadAction = "upload"
	ActionSkip   UploadAction = "skip"
	ActionError  UploadAction = "error"
)

// UploadPlanEntry represents a planned upload operation
type UploadPlanEntry struct {
	Entry  manifest.Entry `json:"entry"`
	Action UploadAction   `json:"action"`
	Error  string         `json:"error,omitempty"`
}

// UploadPlan represents a collection of upload plan entries
type UploadPlan struct {
	Entries []UploadPlanEntry `json:"entries"`
}

// PrintUploadPlan prints a human-readable upload plan
func PrintUploadPlan(plan []UploadPlanEntry) {
	fmt.Printf("\nUpload Plan:\n")
	fmt.Printf("============\n")

	uploadCount := 0
	skipCount := 0
	errorCount := 0
	var totalSize int64

	for _, entry := range plan {
		switch entry.Action {
		case ActionUpload:
			uploadCount++
			totalSize += entry.Entry.FileSize
			fmt.Printf("UPLOAD: %s -> %s (%s)\n",
				filepath.Base(entry.Entry.SourcePath),
				entry.Entry.TargetPath,
				humanizeBytes(entry.Entry.FileSize))
		case ActionSkip:
			skipCount++
			fmt.Printf("SKIP:   %s (already exists)\n",
				filepath.Base(entry.Entry.SourcePath))
		case ActionError:
			errorCount++
			fmt.Printf("ERROR:  %s (%s)\n",
				filepath.Base(entry.Entry.SourcePath),
				entry.Error)
		}
	}

	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Upload: %d files (%s)\n", uploadCount, humanizeBytes(totalSize))
	fmt.Printf("  Skip:   %d files\n", skipCount)
	if errorCount > 0 {
		fmt.Printf("  Error:  %d files\n", errorCount)
	}
}

// testRemoteConnectivity tests basic connectivity to the remote storage
func (c *Client) testRemoteConnectivity(baseRemote string) error {
	if c.dryRun {
		c.logDebug("dry-run connectivity test skipped", "base_remote", baseRemote)
		return nil
	}

	// Use rclone lsd to test basic connectivity (list directories)
	args := []string{"lsd", baseRemote}

	cmd := exec.Command("rclone", args...)
	c.logDebug("testing remote connectivity", "command", cmd.String())

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		stderrStr := stderr.String()
		stdoutStr := stdout.String()
		c.logError("remote connectivity test failed", "error", err, "stderr", stderrStr, "stdout", stdoutStr, "base_remote", baseRemote)
		return fmt.Errorf("remote connectivity test failed for %s: %w (stderr: %s)", baseRemote, err, stderrStr)
	}

	output := strings.TrimSpace(stdout.String())
	c.logDebug("remote connectivity confirmed", "base_remote", baseRemote, "directories", output)
	return nil
}

// testRemoteWriteCapabilityStartup tests remote write capability during startup using extraction metadata
func (c *Client) testRemoteWriteCapabilityStartup() error {
	if c.dryRun {
		c.logDebug("dry-run write capability test skipped during startup")
		return nil
	}

	// Find the extraction-metadata.json file from the backup path
	metadataPath := c.findExtractionMetadataFile()
	if metadataPath == "" {
		c.logDebug("no extraction metadata file found, skipping write capability test")
		return nil
	}

	// Get timestamp from the metadata file's completed_at field
	timestamp, err := c.getTimestampFromMetadata(metadataPath)
	if err != nil {
		c.logWarn("failed to get timestamp from metadata, using current time", "error", err)
		timestamp = time.Now().UTC().Format("2006-01-02T15-04-05Z")
	}

	metadataFileName := fmt.Sprintf("extraction-metadata-%s.json", timestamp)

	// Construct remote path: use the target remote path + metadata/ + filename
	// This puts metadata under the same path as the uploaded photos
	metadataRemotePath := c.buildRemotePath("metadata/" + metadataFileName)

	c.logDebug("uploading extraction metadata to remote", "local_path", metadataPath, "remote_path", metadataRemotePath)

	// Try to upload the metadata file using rclone copyto
	args := []string{"copyto", metadataPath, metadataRemotePath}

	cmd := exec.Command("rclone", args...)
	c.logDebug("testing remote write capability with extraction metadata", "command", cmd.String())

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	if err != nil {
		stderrStr := stderr.String()
		stdoutStr := stdout.String()
		c.logError("extraction metadata upload failed", "error", err, "stderr", stderrStr, "stdout", stdoutStr, "remote_path", metadataRemotePath)
		return fmt.Errorf("extraction metadata upload failed for %s: %w (stderr: %s)", metadataRemotePath, err, stderrStr)
	}

	c.logDebug("extraction metadata upload successful", "remote_path", metadataRemotePath)

	// Try to verify the metadata file exists
	verifyArgs := []string{"lsf", metadataRemotePath}
	verifyCmd := exec.Command("rclone", verifyArgs...)

	var verifyStdout, verifyStderr bytes.Buffer
	verifyCmd.Stdout = &verifyStdout
	verifyCmd.Stderr = &verifyStderr

	verifyErr := verifyCmd.Run()
	if verifyErr != nil {
		c.logError("extraction metadata verification failed", "error", verifyErr, "stderr", verifyStderr.String(), "remote_path", metadataRemotePath)
	} else {
		c.logDebug("extraction metadata verification successful", "remote_path", metadataRemotePath, "output", strings.TrimSpace(verifyStdout.String()))
	}

	// Don't clean up the metadata file - it's useful to keep!
	c.logInfo("extraction metadata backup created", "remote_path", metadataRemotePath)

	return nil
}

// humanizeBytes converts bytes to human readable format
func humanizeBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
