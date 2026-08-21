package db

import (
	"path/filepath"
	"testing"
)

// TestJobFullScheduleRoundTrip proves the full_schedule column writes and
// reads back (issue #322). Without it the column could silently fail to
// persist, leaving a user's scheduled full backup ignored after upgrade.
func TestJobFullScheduleRoundTrip(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	id, err := d.CreateJob(Job{Name: "inc-full", BackupTypeChain: "incremental", FullSchedule: "0 3 * * 0"})
	if err != nil {
		t.Fatal(err)
	}

	got, err := d.GetJob(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.FullSchedule != "0 3 * * 0" {
		t.Errorf("FullSchedule = %q, want %q", got.FullSchedule, "0 3 * * 0")
	}

	// Update clears it and the cleared value round-trips.
	got.FullSchedule = ""
	if err := d.UpdateJob(got); err != nil {
		t.Fatal(err)
	}
	reloaded, err := d.GetJob(id)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.FullSchedule != "" {
		t.Errorf("FullSchedule after clear = %q, want empty", reloaded.FullSchedule)
	}
}
