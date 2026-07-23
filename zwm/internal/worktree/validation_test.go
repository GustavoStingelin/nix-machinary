package worktree_test

import (
	"errors"
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

func TestManagedWorktreePath_sanitizes_and_hashes_identity_deterministically(t *testing.T) {
	// Given
	root := worktree.Path("/managed root/project")

	// When
	path := worktree.ManagedWorktreePath(root, "feature/with-slash")

	// Then
	require.Equal(t, worktree.Path("/managed root/project/feature-with-slash-95ec24a1"), path)
	require.Equal(t, "feature-with-slash", worktree.ManagedDisplay("feature/with-slash"))
	require.Equal(t, "project", worktree.ManagedDisplay(" \n///\t"))
	require.Equal(t, "95ec24a1ed17ada53f3aaa002f81d2ca38bfdde5b28a3c408e7603fd7155f26b", worktree.IdentityHash("feature/with-slash"))
	require.NotEqual(t,
		worktree.ManagedWorktreePath(root, "feature/with-slash"),
		worktree.ManagedWorktreePath(root, "feature-with-slash"),
	)
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
