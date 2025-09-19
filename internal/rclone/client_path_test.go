package rclone

import "testing"

func TestBuildRemotePath(t *testing.T) {
	c := &Client{remote: "GoogleDriveRemote:Backups/iPhone"}
	tests := []struct{ sub, expect string }{
		{"", "GoogleDriveRemote:Backups/iPhone"},
		{"2024/04/photos", "GoogleDriveRemote:Backups/iPhone/2024/04/photos"},
		{"/2024/04/photos/", "GoogleDriveRemote:Backups/iPhone/2024/04/photos"},
		{"\\2024\\04\\photos\\", "GoogleDriveRemote:Backups/iPhone/2024/04/photos"},
		{"2024//05//photos", "GoogleDriveRemote:Backups/iPhone/2024/05/photos"},
	}
	for _, tt := range tests {
		got := c.buildRemotePath(tt.sub)
		if got != tt.expect {
			// Show expectation clearly
			t.Errorf("buildRemotePath(%q) = %q want %q", tt.sub, got, tt.expect)
		}
	}

	c2 := &Client{remote: "GoogleDriveRemote"}
	if got := c2.buildRemotePath("2024/05/photos"); got != "GoogleDriveRemote:2024/05/photos" {
		t.Fatalf("unexpected: %s", got)
	}

	// Absolute path remote (e.g., sftp: or local path based remote)
	c3 := &Client{remote: "sftp:/absolute/base"}
	if got := c3.buildRemotePath("2024/06/photos"); got != "sftp:/absolute/base/2024/06/photos" {
		t.Fatalf("unexpected absolute join: %s", got)
	}
	if got := c3.buildRemotePath(""); got != "sftp:/absolute/base" {
		t.Fatalf("unexpected absolute base only: %s", got)
	}
}
