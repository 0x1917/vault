package handlers

import (
	"testing"

	"github.com/ruaan-deysel/vault/internal/config"
)

// TestValidateJobEnumContainerMode pins the container_mode allow-list to the
// values the rest of the stack actually uses. The API list carried
// "all_at_once" while the UI, the runner and config.ContainerStopAll all use
// "stop_all", so saving a job in Batch mode was rejected outright (issue #261).
func TestValidateJobEnumContainerMode(t *testing.T) {
	cases := []struct {
		value   string
		wantErr bool
	}{
		{string(config.ContainerOneByOne), false},
		{string(config.ContainerStopAll), false},
		{"", false}, // omitted — the DB column default applies
		{"nonsense", true},
		// Migrated away by dataMigrations at upgrade, not accepted here.
		{"all_at_once", true},
	}

	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			err := validateJobEnum("container_mode", tc.value)
			if tc.wantErr && err == nil {
				t.Fatalf("validateJobEnum(container_mode, %q) = nil, want error", tc.value)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateJobEnum(container_mode, %q) = %v, want nil", tc.value, err)
			}
		})
	}
}

// TestJobEnumsMatchConfigConstants guards the whole allow-list against the same
// class of drift: every value the config package declares as a valid mode must
// be accepted by the API validator.
func TestJobEnumsMatchConfigConstants(t *testing.T) {
	for field, want := range map[string][]string{
		"container_mode": {string(config.ContainerOneByOne), string(config.ContainerStopAll)},
	} {
		for _, v := range want {
			if err := validateJobEnum(field, v); err != nil {
				t.Errorf("%s: config declares %q but the API validator rejects it: %v", field, v, err)
			}
		}
	}
}

// TestValidateJobEnumContainerScope pins the container_scope allow-list to the
// two selection modes the runner understands (#324): "all" (every live
// container, reconciled per run) and "custom" (explicit selection). Without
// this entry a job saved with the new wizard option would be rejected.
func TestValidateJobEnumContainerScope(t *testing.T) {
	cases := []struct {
		value   string
		wantErr bool
	}{
		{"all", false},
		{"custom", false},
		{"", false}, // omitted — the runner treats non-"all" as custom
		{"nonsense", true},
	}

	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			err := validateJobEnum("container_scope", tc.value)
			if tc.wantErr && err == nil {
				t.Fatalf("validateJobEnum(container_scope, %q) = nil, want error", tc.value)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateJobEnum(container_scope, %q) = %v, want nil", tc.value, err)
			}
		})
	}
}
