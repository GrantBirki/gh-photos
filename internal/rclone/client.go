package rclone

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/grantbirki/gh-photos/internal/logger"
	"github.com/grantbirki/gh-photos/internal/manifest"
)

// Client wraps rclone operations
type Client struct {
	remote       string
	parallel     int
	verify       bool
	dryRun       bool
	skipExisting bool
	logger       *logger.Logger
	logLevel     string
}

// NewClient creates a new rclone client
func NewClient(remote string, parallel int, verify, dryRun, skipExisting bool, logger *logger.Logger, logLevel string) *Client {
	return &Client{
		remote:       remote,
		parallel:     parallel,
		verify:       verify,
		dryRun:       dryRun,
		skipExisting: skipExisting,
		logger:       logger,
		logLevel:     logLevel,
	}
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
	args = append(args, fmt.Sprintf("%s:%s", c.remote, entry.TargetPath))

	// Execute rclone command
	cmd := exec.CommandContext(ctx, "rclone", args...)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rclone upload failed: %w", err)
	}

	return nil
}

// ProgressCallback provides upload progress updates
type ProgressCallback func(completed, total int, currentFile string)

// UploadBatch uploads multiple entries using efficient batch operations with progress reporting
// This single method handles all upload operations regardless of dataset size for optimal performance
func (c *Client) UploadBatch(ctx context.Context, entries []manifest.Entry, updateCallback func(int, manifest.OperationStatus, string), progressCallback ProgressCallback) error {
	if len(entries) == 0 {
		return nil
	}
	// Handle dry run mode - just report what would be uploaded
	if c.dryRun {
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
		return nil
	}

	// Determine optimal chunk size based on dataset size for better progress feedback
	chunkSize := 200 // Default chunk size for good progress reporting
	if len(entries) <= 50 {
		chunkSize = len(entries) // Single chunk for small datasets
	} else if len(entries) <= 500 {
		chunkSize = 100 // Medium chunks for moderate datasets
	}

	// Process entries in chunks
	for i := 0; i < len(entries); i += chunkSize {
		end := i + chunkSize
		if end > len(entries) {
			end = len(entries)
		}

		chunk := entries[i:end]
		if err := c.uploadChunk(ctx, chunk, entries, i, updateCallback, progressCallback); err != nil {
			return err
		}
	}

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

	completed := baseIndex
	total := len(allEntries)

	// Process each directory group
	for targetDir, groupEntries := range dirGroups {
		// Create temporary directory for this batch
		tempDir, err := os.MkdirTemp("", "gh-photos-batch-*")
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
		defer os.RemoveAll(tempDir)

		// Create directory structure and copy files to temp location
		for _, entry := range groupEntries {
			if progressCallback != nil {
				progressCallback(completed, total, fmt.Sprintf("Preparing %s", filepath.Base(entry.SourcePath)))
			}

			// Create target directory structure in temp
			tempTargetPath := filepath.Join(tempDir, entry.TargetPath)
			if err := os.MkdirAll(filepath.Dir(tempTargetPath), 0755); err != nil {
				return fmt.Errorf("failed to create temp directory structure: %w", err)
			}

			// Create symlink to avoid copying large files
			if err := os.Symlink(entry.SourcePath, tempTargetPath); err != nil {
				return fmt.Errorf("failed to create symlink: %w", err)
			}
		}

		// Build rclone command for batch upload
		args := []string{"copy", tempDir, fmt.Sprintf("%s:%s", c.remote, targetDir)}

		if c.skipExisting {
			args = append(args, "--ignore-existing")
		}

		if c.verify {
			args = append(args, "--check-first")
		}

		// Add progress reporting
		args = append(args, "--progress", "--stats-one-line")

		// Execute batch rclone command
		cmd := exec.CommandContext(ctx, "rclone", args...)

		// Capture output for progress parsing
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("failed to create stdout pipe: %w", err)
		}

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start rclone: %w", err)
		}

		// Parse progress output
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if progressCallback != nil && strings.Contains(line, "Transferred:") {
				progressCallback(completed, total, fmt.Sprintf("Uploading batch (%d files)", len(groupEntries)))
			}
		}

		if err := cmd.Wait(); err != nil {
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

	cmd := exec.CommandContext(ctx, "rclone", "lsf", fullPath)
	output, err := cmd.Output()

	if err != nil {
		// If the command fails, the file doesn't exist
		return false, nil
	}

	return strings.TrimSpace(string(output)) != "", nil
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
		return c.listAllRemoteFiles(ctx)
	}

	// Otherwise, list each directory separately to be more efficient
	for dir := range dirs {
		remotePath := fmt.Sprintf("%s:%s", c.remote, dir)
		cmd := exec.CommandContext(ctx, "rclone", "lsf", remotePath, "-R")
		output, err := cmd.Output()

		if err != nil {
			// Directory might not exist, continue with others
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
	}

	return existingFiles, nil
}

// listAllRemoteFiles lists all files on the remote recursively
func (c *Client) listAllRemoteFiles(ctx context.Context) (map[string]bool, error) {
	remotePath := fmt.Sprintf("%s:", c.remote)
	cmd := exec.CommandContext(ctx, "rclone", "lsf", remotePath, "-R")
	output, err := cmd.Output()

	if err != nil {
		return nil, fmt.Errorf("failed to list remote files: %w", err)
	}

	existingFiles := make(map[string]bool)
	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, file := range files {
		if file != "" {
			existingFiles[file] = true
		}
	}

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

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	return nil
}

// GetRemoteSize gets the size of a remote file
func (c *Client) GetRemoteSize(ctx context.Context, remotePath string) (int64, error) {
	fullPath := fmt.Sprintf("%s:%s", c.remote, remotePath)

	cmd := exec.CommandContext(ctx, "rclone", "size", fullPath, "--json")
	output, err := cmd.Output()

	if err != nil {
		return 0, fmt.Errorf("failed to get remote size: %w", err)
	}

	// Parse the JSON output (simplified)
	// In a real implementation, you'd properly parse the JSON
	if strings.Contains(string(output), "\"bytes\":") {
		return 0, nil // Placeholder - would parse actual size
	}

	return 0, nil
}

// ValidateRcloneInstallation checks if rclone is available and properly configured
func ValidateRcloneInstallation() error {
	// Check if rclone is in PATH
	_, err := exec.LookPath("rclone")
	if err != nil {
		return fmt.Errorf("rclone not found in PATH: %w", err)
	}

	// Check rclone version
	cmd := exec.Command("rclone", "version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get rclone version: %w", err)
	}

	version := string(output)
	if !strings.Contains(version, "rclone") {
		return fmt.Errorf("unexpected rclone version output: %s", version)
	}

	return nil
}

// ValidateRemote checks if the specified remote is configured
func ValidateRemote(remote string) error {
	cmd := exec.Command("rclone", "listremotes")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list rclone remotes: %w", err)
	}

	remotes := strings.Split(string(output), "\n")
	remoteName := strings.Split(remote, ":")[0] + ":"

	for _, r := range remotes {
		if strings.TrimSpace(r) == remoteName {
			return nil
		}
	}

	return fmt.Errorf("remote '%s' not found in configured remotes", remoteName)
}

// ValidateRemoteAuthentication tests if the remote is accessible and authenticated
func ValidateRemoteAuthentication(remote string) error {
	// Test authentication by trying to list the root directory
	cmd := exec.Command("rclone", "lsf", remote+":", "--max-depth", "1")
	cmd.Stderr = nil // Suppress stderr to avoid credential prompts

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remote authentication failed - credentials may be expired or invalid: %w", err)
	}

	return nil
}

// CreateUploadPlan creates a plan for uploading assets
func (c *Client) CreateUploadPlan(ctx context.Context, entries []manifest.Entry) ([]UploadPlanEntry, error) {
	plan := make([]UploadPlanEntry, 0, len(entries))

	// If skip existing is enabled, batch check all remote files at once
	var existingFiles map[string]bool
	if c.skipExisting {
		if c.logger != nil {
			c.logger.Info("Checking existing files on remote (this may take a moment)...", "files_to_check", len(entries))
		}

		var err error
		existingFiles, err = c.BatchCheckRemoteExists(ctx, entries)
		if err != nil {
			if c.logger != nil {
				c.logger.Warn("Failed to batch check remote files, falling back to individual checks", "error", err.Error())
			}
			// Fall back to individual checks if batch fails
			existingFiles = nil
		} else if c.logger != nil {
			existingCount := len(existingFiles)
			c.logger.Info("Remote file check complete", "existing_files", existingCount)
		}
	}

	// Process each entry
	for i, entry := range entries {
		planEntry := UploadPlanEntry{
			Entry:  entry,
			Action: ActionUpload,
		}

		if c.skipExisting {
			var exists bool
			var err error

			if existingFiles != nil {
				// Use batch result
				exists = existingFiles[entry.TargetPath]
			} else {
				// Fall back to individual check
				exists, err = c.CheckRemoteExists(ctx, entry.TargetPath)
				if err != nil {
					planEntry.Action = ActionError
					planEntry.Error = err.Error()
					plan = append(plan, planEntry)
					continue
				}
			}

			if exists {
				planEntry.Action = ActionSkip
				if c.logger != nil && c.logLevel == "debug" {
					c.logger.Debug("Skipping existing file",
						"file", filepath.Base(entry.SourcePath),
						"remote", c.remote,
						"target", entry.TargetPath)
				}
			}
		}

		plan = append(plan, planEntry)

		// Progress reporting for large uploads
		if c.logger != nil && len(entries) > 100 && (i+1)%500 == 0 {
			c.logger.Info("Upload plan progress", "processed", i+1, "total", len(entries))
		}
	}

	return plan, nil
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

// result represents the result of an upload operation
type result struct {
	index int
	err   error
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
