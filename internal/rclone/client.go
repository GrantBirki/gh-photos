package rclone

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/grantbirki/gh-photos/internal/manifest"
)

// Client wraps rclone operations
type Client struct {
	remote       string
	parallel     int
	verify       bool
	dryRun       bool
	skipExisting bool
}

// NewClient creates a new rclone client
func NewClient(remote string, parallel int, verify, dryRun, skipExisting bool) *Client {
	return &Client{
		remote:       remote,
		parallel:     parallel,
		verify:       verify,
		dryRun:       dryRun,
		skipExisting: skipExisting,
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

// UploadBatch uploads multiple entries in parallel
func (c *Client) UploadBatch(ctx context.Context, entries []manifest.Entry, updateCallback func(int, manifest.OperationStatus, string)) error {
	if len(entries) == 0 {
		return nil
	}

	// Create a semaphore to limit concurrent uploads
	semaphore := make(chan struct{}, c.parallel)
	results := make(chan result, len(entries))

	// Start uploads
	for i, entry := range entries {
		go func(index int, e manifest.Entry) {
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			err := c.UploadEntry(ctx, e)
			results <- result{index: index, err: err}
		}(i, entry)
	}

	// Collect results
	for i := 0; i < len(entries); i++ {
		select {
		case res := <-results:
			if res.err != nil {
				updateCallback(res.index, manifest.StatusFailed, res.err.Error())
			} else {
				updateCallback(res.index, manifest.StatusUploaded, "")
			}
		case <-ctx.Done():
			return ctx.Err()
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

// CreateUploadPlan creates a plan for uploading assets
func (c *Client) CreateUploadPlan(ctx context.Context, entries []manifest.Entry) ([]UploadPlanEntry, error) {
	plan := make([]UploadPlanEntry, 0, len(entries))

	for _, entry := range entries {
		planEntry := UploadPlanEntry{
			Entry:  entry,
			Action: ActionUpload,
		}

		if c.skipExisting {
			exists, err := c.CheckRemoteExists(ctx, entry.TargetPath)
			if err != nil {
				planEntry.Action = ActionError
				planEntry.Error = err.Error()
			} else if exists {
				planEntry.Action = ActionSkip
			}
		}

		plan = append(plan, planEntry)
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
