package runner

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/ruaan-deysel/vault/internal/db"
	"github.com/ruaan-deysel/vault/internal/engine"
)

// TestExpandContainerScopeCustomIsIdentity: a custom-scoped job must pass its
// stored items through untouched (and never touch Docker).
func TestExpandContainerScopeCustomIsIdentity(t *testing.T) {
	items := []db.JobItem{{ItemType: "container", ItemName: "plex"}}
	got, err := expandContainerScope(db.Job{ContainerScope: "custom"}, items)
	if err != nil {
		t.Fatalf("expandContainerScope custom: %v", err)
	}
	if len(got) != 1 || got[0].ItemName != "plex" {
		t.Fatalf("custom scope should return items unchanged, got %+v", got)
	}
}

// TestExpandContainerScopeAllReplacesContainerItems: an all-scoped job rebuilds
// its container items from discovery — new containers appear, deleted ones drop,
// and non-container items are preserved in their original order.
func TestExpandContainerScopeAllReplacesContainerItems(t *testing.T) {
	original := listContainersFn
	defer func() { listContainersFn = original }()
	listContainersFn = func() ([]engine.BackupItem, error) {
		return []engine.BackupItem{
			{Name: "sonarr", Type: "container", Settings: map[string]any{"id": "id-sonarr", "image": "linuxserver/sonarr", "state": "running"}},
			{Name: "plex", Type: "container", Settings: map[string]any{"id": "id-plex", "image": "plexinc/pms", "state": "running"}},
		}, nil
	}

	items := []db.JobItem{
		{ItemType: "folder", ItemName: "flash", ItemID: "/boot"},
		{ItemType: "container", ItemName: "old-removed", ItemID: "id-old", Settings: `{"id":"id-old"}`},
	}

	got, err := expandContainerScope(db.Job{ContainerScope: "all"}, items)
	if err != nil {
		t.Fatalf("expandContainerScope all: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected folder + 2 discovered containers = 3 items, got %d: %+v", len(got), got)
	}
	if got[0].ItemType != "folder" || got[0].ItemName != "flash" {
		t.Fatalf("expected folder first, got %+v", got[0])
	}
	if got[1].ItemType != "container" || got[1].ItemName != "plex" {
		t.Fatalf("expected plex second (sorted by name), got %+v", got[1])
	}
	if got[2].ItemType != "container" || got[2].ItemName != "sonarr" {
		t.Fatalf("expected sonarr third (sorted by name), got %+v", got[2])
	}

	var s map[string]any
	if err := json.Unmarshal([]byte(got[1].Settings), &s); err != nil {
		t.Fatalf("settings JSON: %v", err)
	}
	if s["id"] != "id-plex" || s["image"] != "plexinc/pms" || s["state"] != "running" {
		t.Fatalf("unexpected settings: %+v", s)
	}
	if got[1].ItemID != "id-plex" {
		t.Fatalf("expected ItemID id-plex, got %q", got[1].ItemID)
	}
}

// TestExpandContainerScopeAllDegradesOnDiscoveryError: when Docker discovery
// fails (transient outage), the original items are returned unchanged along with
// an error — never silently stripping the container list.
func TestExpandContainerScopeAllDegradesOnDiscoveryError(t *testing.T) {
	original := listContainersFn
	defer func() { listContainersFn = original }()
	listContainersFn = func() ([]engine.BackupItem, error) {
		return nil, errors.New("docker unavailable")
	}

	items := []db.JobItem{{ItemType: "container", ItemName: "plex"}}
	got, err := expandContainerScope(db.Job{ContainerScope: "all"}, items)
	if err == nil {
		t.Fatal("expected an error when discovery fails")
	}
	if len(got) != 1 || got[0].ItemName != "plex" {
		t.Fatalf("on discovery error, original items should be returned unchanged, got %+v", got)
	}
}
