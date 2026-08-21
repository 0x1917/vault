package runner

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/ruaan-deysel/vault/internal/db"
	"github.com/ruaan-deysel/vault/internal/engine"
)

// listContainersFn is the container-discovery seam for expandContainerScope.
// It is swapped in tests to avoid constructing a real Docker client. The real
// implementation reuses engine.ContainerHandler.ListItems, the same source the
// discovery endpoint and stale-detection inventory use, so the expanded set
// always agrees with what the engine can actually back up.
var listContainersFn = func() ([]engine.BackupItem, error) {
	ch, err := engine.NewContainerHandler()
	if err != nil {
		return nil, err
	}
	return ch.ListItems()
}

// expandContainerScope returns the item set a job should back up. For an
// "all"-scoped job it rebuilds the container items from live discovery and
// keeps every non-container item in place; for any other scope it returns the
// stored items unchanged. On a discovery error it returns the original items
// together with the error so the caller can log and proceed — a transient
// Docker outage must never silently strip the container list (#324).
func expandContainerScope(job db.Job, items []db.JobItem) ([]db.JobItem, error) {
	if job.ContainerScope != "all" {
		return items, nil
	}
	discovered, err := listContainersFn()
	if err != nil {
		return items, fmt.Errorf("discovering containers for all-scope job: %w", err)
	}
	return mergeAllContainers(items, discovered), nil
}

// mergeAllContainers returns the non-container items in their original order,
// followed by a container item for every discovered container. Existing
// container items are discarded entirely — the live set is the source of truth,
// so new containers are added and deleted ones removed automatically.
func mergeAllContainers(items []db.JobItem, discovered []engine.BackupItem) []db.JobItem {
	out := make([]db.JobItem, 0, len(items)+len(discovered))
	for _, it := range items {
		if it.ItemType != "container" {
			out = append(out, it)
		}
	}
	out = append(out, containerItemsFromDiscovered(discovered)...)
	return out
}

// containerItemsFromDiscovered converts engine-discovered containers into job
// items, carrying the id/image/state settings the backup loop reads. The
// database_kind hint is preserved so a database-dump toggle stays available for
// auto-included containers on subsequent edits.
func containerItemsFromDiscovered(discovered []engine.BackupItem) []db.JobItem {
	items := make([]db.JobItem, 0, len(discovered))
	for _, c := range discovered {
		id, _ := c.Settings["id"].(string)
		settings := map[string]any{
			"id":    c.Settings["id"],
			"image": c.Settings["image"],
			"state": c.Settings["state"],
		}
		if kind, ok := c.Settings["database_kind"]; ok {
			settings["database_kind"] = kind
		}
		raw, _ := json.Marshal(settings)
		items = append(items, db.JobItem{
			ItemType: "container",
			ItemName: c.Name,
			ItemID:   id,
			Settings: string(raw),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ItemName < items[j].ItemName })
	return items
}
