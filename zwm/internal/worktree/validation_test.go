package worktree_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
	"github.com/stretchr/testify/require"
)

func TestValidateTarget_classifies_every_registration_and_checkout_state_when_records_are_known(t *testing.T) {
	managedPath := worktree.Path("/managed/feature-topic")
	branch := worktree.Branch("feature/topic")
	record := func(path worktree.Path, recordBranch worktree.Branch) worktree.Record {
		return worktree.Record{
			Path:   path,
			Head:   worktree.OID(sha1),
			Branch: worktree.LocalRef(recordBranch),
			State:  worktree.HeadBranch,
		}
	}

	tests := []struct {
		name             string
		input            worktree.TargetInput
		wantRegistration worktree.RegistrationState
		wantBranch       worktree.BranchState
		accepted         bool
	}{
		{
			name:             "available unregistered",
			input:            worktree.TargetInput{Branch: branch, ManagedPath: managedPath},
			wantRegistration: worktree.RegistrationAvailable,
			wantBranch:       worktree.BranchUnregistered,
			accepted:         true,
		},
		{
			name:             "occupied unregistered",
			input:            worktree.TargetInput{Branch: branch, ManagedPath: managedPath, PathOccupied: true},
			wantRegistration: worktree.RegistrationOccupied,
			wantBranch:       worktree.BranchUnregistered,
		},
		{
			name: "reusable managed",
			input: worktree.TargetInput{
				Branch: branch, ManagedPath: managedPath, Records: []worktree.Record{
					record(worktree.Path("/source"), worktree.Branch("main")),
					record(managedPath, branch),
				},
			},
			wantRegistration: worktree.RegistrationReusable,
			wantBranch:       worktree.BranchManaged,
			accepted:         true,
		},
		{
			name: "prunable managed registration",
			input: worktree.TargetInput{
				Branch: branch, ManagedPath: managedPath, Records: []worktree.Record{
					record(worktree.Path("/source"), worktree.Branch("main")),
					{
						Path: managedPath, Head: worktree.OID(sha1), Branch: worktree.LocalRef(branch), State: worktree.HeadBranch,
						Prunable: true, PrunableReason: "gitdir file points to non-existent location",
					},
				},
			},
			wantRegistration: worktree.RegistrationInvalid,
			wantBranch:       worktree.BranchManaged,
		},
		{
			name: "detached registration",
			input: worktree.TargetInput{
				Branch: branch, ManagedPath: managedPath,
				Records: []worktree.Record{{Path: managedPath, Head: worktree.OID(sha1), State: worktree.HeadDetached}},
			},
			wantRegistration: worktree.RegistrationDetached,
			wantBranch:       worktree.BranchUnregistered,
		},
		{
			name: "mismatched registration",
			input: worktree.TargetInput{
				Branch: branch, ManagedPath: managedPath, Records: []worktree.Record{record(managedPath, worktree.Branch("other/topic"))},
			},
			wantRegistration: worktree.RegistrationMismatched,
			wantBranch:       worktree.BranchUnregistered,
		},
		{
			name: "primary checkout",
			input: worktree.TargetInput{
				Branch: branch, ManagedPath: managedPath, Records: []worktree.Record{record(worktree.Path("/source"), branch)},
			},
			wantRegistration: worktree.RegistrationAvailable,
			wantBranch:       worktree.BranchPrimary,
		},
		{
			name: "unmanaged checkout",
			input: worktree.TargetInput{
				Branch: branch, ManagedPath: managedPath, Records: []worktree.Record{
					record(worktree.Path("/source"), worktree.Branch("main")),
					record(worktree.Path("/unmanaged linked\nworktree"), branch),
				},
			},
			wantRegistration: worktree.RegistrationAvailable,
			wantBranch:       worktree.BranchUnmanaged,
		},
		{
			name: "duplicate branch registrations",
			input: worktree.TargetInput{
				Branch: branch, ManagedPath: managedPath, Records: []worktree.Record{
					record(worktree.Path("/first"), branch),
					record(worktree.Path("/second"), branch),
				},
			},
			wantRegistration: worktree.RegistrationAvailable,
			wantBranch:       worktree.BranchDuplicate,
		},
		{
			name: "duplicate path registrations",
			input: worktree.TargetInput{
				Branch: branch, ManagedPath: managedPath, Records: []worktree.Record{
					record(managedPath, branch),
					record(managedPath, worktree.Branch("other/topic")),
				},
			},
			wantRegistration: worktree.RegistrationDuplicate,
			wantBranch:       worktree.BranchDuplicate,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := worktree.ValidateTarget(test.input)
			_, err := got.AcceptedPath()

			// Then
			require.Equal(t, test.wantRegistration, got.Registration)
			require.Equal(t, test.wantBranch, got.Branch)
			if test.accepted {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, worktree.ErrInvalidTarget)
		})
	}
}

func TestManagedWorktreePath_builds_concise_leaf_when_identity_fits_limit(t *testing.T) {
	// Given
	root := worktree.Path("/managed root/project")
	tests := []struct {
		name     string
		identity string
		wantLeaf string
	}{
		{name: "short branch", identity: "mau", wantLeaf: "mau"},
		{name: "slash and space branch", identity: "feature/ready now", wantLeaf: "feature-ready-now"},
		{name: "numeric pull request", identity: "pr-1185", wantLeaf: "pr-1185"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			path := worktree.ManagedWorktreePath(root, test.identity)

			// Then
			require.Equal(t, worktree.Path(filepath.Join(string(root), test.wantLeaf)), path)
		})
	}
}

func TestManagedWorktreePath_suffixes_cropped_leaf_deterministically_when_identity_exceeds_limit(t *testing.T) {
	// Given
	root := worktree.Path("/managed root/project")
	sharedPrefix := "feature/" + strings.Repeat("shared-prefix-", 8)
	firstBranch := sharedPrefix + "first"
	secondBranch := sharedPrefix + "second"

	// When
	firstPath := worktree.ManagedWorktreePath(root, firstBranch)
	secondPath := worktree.ManagedWorktreePath(root, secondBranch)
	firstLeaf := filepath.Base(string(firstPath))
	secondLeaf := filepath.Base(string(secondPath))

	// Then
	require.LessOrEqual(t, len(firstLeaf), 64)
	require.LessOrEqual(t, len(secondLeaf), 64)
	require.True(t, strings.HasPrefix(firstLeaf, "feature-shared-prefix"))
	require.True(t, strings.HasPrefix(secondLeaf, "feature-shared-prefix"))
	require.NotEqual(t, firstLeaf, secondLeaf)
	require.Equal(t, firstPath, worktree.ManagedWorktreePath(root, firstBranch))
	require.Equal(t, secondPath, worktree.ManagedWorktreePath(root, secondBranch))
}

func TestManagedDisplay_and_identity_hash_remain_stable_for_output_identity_helpers(t *testing.T) {
	// Then
	require.Equal(t, "feature-with-slash", worktree.ManagedDisplay("feature/with-slash"))
	require.Equal(t, "project", worktree.ManagedDisplay(" \n///\t"))
	require.Equal(t, "95ec24a1ed17ada53f3aaa002f81d2ca38bfdde5b28a3c408e7603fd7155f26b", worktree.IdentityHash("feature/with-slash"))
}

func TestValidation_retains_typed_invalid_target_error_when_combination_is_rejected(t *testing.T) {
	// Given
	validation := worktree.ValidateTarget(worktree.TargetInput{
		Branch:       worktree.Branch("feature/topic"),
		ManagedPath:  worktree.Path("/managed/feature-topic"),
		PathOccupied: true,
	})

	// When
	_, err := validation.AcceptedPath()

	// Then
	var targetError *worktree.InvalidTargetError
	require.ErrorAs(t, err, &targetError)
	require.True(t, errors.Is(err, worktree.ErrInvalidTarget))
}
