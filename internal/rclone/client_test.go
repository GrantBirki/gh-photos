package rclone

import (
	"testing"
)

func TestValidateRemoteAuthentication(t *testing.T) {
	// Test with a non-existent remote to ensure it fails appropriately
	err := ValidateRemoteAuthentication("nonexistent-remote", nil)
	if err == nil {
		t.Error("Expected error for non-existent remote, but got nil")
	}
}

func TestValidateRemote(t *testing.T) {
	// Test that function exists and handles invalid remotes
	err := ValidateRemote("nonexistent-remote", nil)
	if err == nil {
		t.Error("Expected error for non-existent remote, but got nil")
	}
}
