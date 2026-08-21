package scheduler

import (
	"sync/atomic"
	"testing"

	"github.com/ruaan-deysel/vault/internal/db"
)

// TestSchedulerFullJob proves an enabled job with a FullSchedule registers a
// full-backup cron entry (issue #322), via the SetFullRunner hook.
func TestSchedulerFullJob(t *testing.T) {
	d := testDB(t)
	destID, _ := d.CreateStorageDestination(db.StorageDestination{Name: "t", Type: "local", Config: "{}"})
	if _, err := d.CreateJob(db.Job{
		Name: "inc", Enabled: true, Schedule: "0 2 * * *",
		BackupTypeChain: "incremental", StorageDestID: destID,
		FullSchedule: "0 3 * * 0",
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	var fullHits atomic.Int32
	s := New(d, func(int64) {})
	s.SetFullRunner(func(int64) { fullHits.Add(1) })
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop()

	if len(s.fullEntries) != 1 {
		t.Errorf("fullEntries = %d, want 1", len(s.fullEntries))
	}
}

// TestSchedulerFullJobBadCron exercises the error branch in addFullJob.
func TestSchedulerFullJobBadCron(t *testing.T) {
	d := testDB(t)
	destID, _ := d.CreateStorageDestination(db.StorageDestination{Name: "t", Type: "local", Config: "{}"})
	if _, err := d.CreateJob(db.Job{
		Name: "bad-full", Enabled: true, Schedule: "0 2 * * *",
		BackupTypeChain: "incremental", StorageDestID: destID,
		FullSchedule: "not-a-cron",
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}
	s := New(d, func(int64) {})
	s.SetFullRunner(func(int64) {})
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop()
	if len(s.fullEntries) != 0 {
		t.Errorf("fullEntries on bad cron = %d, want 0", len(s.fullEntries))
	}
}

// TestSchedulerFullJobLastDay covers the "L" last-day-of-month token branch.
func TestSchedulerFullJobLastDay(t *testing.T) {
	d := testDB(t)
	destID, _ := d.CreateStorageDestination(db.StorageDestination{Name: "t", Type: "local", Config: "{}"})
	if _, err := d.CreateJob(db.Job{
		Name: "monthly-full", Enabled: true, Schedule: "0 2 * * *",
		BackupTypeChain: "differential", StorageDestID: destID,
		FullSchedule: "0 3 L * *",
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}
	s := New(d, func(int64) {})
	s.SetFullRunner(func(int64) {})
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop()
	if len(s.fullLastDayEntries) != 1 {
		t.Errorf("fullLastDayEntries = %d, want 1", len(s.fullLastDayEntries))
	}
}
