package worktree_test

import (
	"errors"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
	"github.com/stretchr/testify/require"
)

const sha1 = "0123456789abcdef0123456789abcdef01234567"

func TestParsePorcelainZ_preserves_embedded_spaces_and_newlines_when_records_are_valid(t *testing.T) {
	// Given
	raw := []byte("worktree /tmp/managed space\nname\x00HEAD " + sha1 + "\x00branch refs/heads/feature/with-space\x00\x00" +
		"worktree /tmp/detached\x00HEAD " + sha1 + "\x00detached\x00\x00")

	// When
	records, err := worktree.ParsePorcelainZ(raw)

	// Then
	require.NoError(t, err)
	require.Equal(t, []worktree.Record{
		{
			Path:   worktree.Path("/tmp/managed space\nname"),
			Head:   worktree.OID(sha1),
			Branch: worktree.Ref("refs/heads/feature/with-space"),
			State:  worktree.HeadBranch,
		},
		{
			Path:  worktree.Path("/tmp/detached"),
			Head:  worktree.OID(sha1),
			State: worktree.HeadDetached,
		},
	}, records)
}

func TestParsePorcelainZ_rejects_malformed_records_when_structure_is_ambiguous(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
	}{
		{name: "empty output", raw: []byte{}},
		{name: "empty record", raw: []byte("\x00")},
		{name: "missing final NUL", raw: []byte("worktree /tmp/a\x00HEAD " + sha1 + "\x00detached")},
		{name: "missing record separator", raw: []byte("worktree /tmp/a\x00HEAD " + sha1 + "\x00detached\x00")},
		{name: "duplicate worktree", raw: []byte("worktree /tmp/a\x00worktree /tmp/b\x00HEAD " + sha1 + "\x00detached\x00\x00")},
		{name: "duplicate HEAD", raw: []byte("worktree /tmp/a\x00HEAD " + sha1 + "\x00HEAD " + sha1 + "\x00detached\x00\x00")},
		{name: "duplicate branch", raw: []byte("worktree /tmp/a\x00HEAD " + sha1 + "\x00branch refs/heads/a\x00branch refs/heads/b\x00\x00")},
		{name: "branch and detached", raw: []byte("worktree /tmp/a\x00HEAD " + sha1 + "\x00branch refs/heads/a\x00detached\x00\x00")},
		{name: "invalid HEAD", raw: []byte("worktree /tmp/a\x00HEAD not-an-object\x00detached\x00\x00")},
		{name: "invalid branch", raw: []byte("worktree /tmp/a\x00HEAD " + sha1 + "\x00branch invalid-ref\x00\x00")},
		{name: "unknown attribute", raw: []byte("worktree /tmp/a\x00HEAD " + sha1 + "\x00detached\x00unknown value\x00\x00")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			_, err := worktree.ParsePorcelainZ(test.raw)

			// Then
			require.ErrorIs(t, err, worktree.ErrMalformedPorcelain)
		})
	}
}

func TestParsePorcelainZ_parses_bare_record_when_Git_marks_it_bare(t *testing.T) {
	// Given
	raw := []byte("worktree /tmp/bare\x00bare\x00\x00")

	// When
	records, err := worktree.ParsePorcelainZ(raw)

	// Then
	require.NoError(t, err)
	require.Equal(t, []worktree.Record{{Path: worktree.Path("/tmp/bare"), State: worktree.HeadBare}}, records)
}

func TestParsePorcelainZ_preserves_prunable_presence_and_optional_reason(t *testing.T) {
	tests := []struct {
		name       string
		attribute  string
		wantReason string
	}{
		{name: "without reason", attribute: "prunable"},
		{name: "with reason", attribute: "prunable gitdir file\npoints to missing location", wantReason: "gitdir file\npoints to missing location"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			raw := []byte("worktree /tmp/stale path\nname\x00HEAD " + sha1 + "\x00branch refs/heads/stale/topic\x00" + test.attribute + "\x00\x00")

			// When
			records, err := worktree.ParsePorcelainZ(raw)

			// Then
			require.NoError(t, err)
			require.Len(t, records, 1)
			require.True(t, records[0].Prunable)
			require.Equal(t, test.wantReason, records[0].PrunableReason)
		})
	}
}

func TestParsePorcelainZ_retains_typed_malformed_error_when_input_is_truncated(t *testing.T) {
	// Given
	raw := []byte("worktree /tmp/a\x00HEAD " + sha1 + "\x00")

	// When
	_, err := worktree.ParsePorcelainZ(raw)

	// Then
	var parseError *worktree.ParseError
	require.ErrorAs(t, err, &parseError)
	require.True(t, errors.Is(err, worktree.ErrMalformedPorcelain))
}
