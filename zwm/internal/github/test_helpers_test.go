package github_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type invocation struct {
	Directory string
	Arguments []string
}

func fakeGH(t *testing.T) (string, string) {
	t.Helper()

	directory := t.TempDir()
	helper := filepath.Join(directory, "gh")
	recordPath := filepath.Join(directory, "invocations")
	require.NoError(t, os.WriteFile(helper, []byte(`#!/bin/sh
if [ -n "${GH_RECORD:-}" ]; then
  printf '%s\037' "$PWD" >> "$GH_RECORD"
  for argument in "$@"; do
    printf '%s\037' "$argument" >> "$GH_RECORD"
  done
  printf '\n' >> "$GH_RECORD"
fi

case "$1:$2" in
  pr:view)
    printf '%s' "${GH_VIEW_STDOUT:-}"
    printf '%s' "${GH_VIEW_STDERR:-}" >&2
    exit "${GH_VIEW_EXIT:-0}"
    ;;
  pr:checkout)
    printf '%s' "${GH_CHECKOUT_STDOUT:-}"
    printf '%s' "${GH_CHECKOUT_STDERR:-}" >&2
    exit "${GH_CHECKOUT_EXIT:-0}"
    ;;
  *)
    printf 'unexpected fake gh command: %s\n' "$*" >&2
    exit 64
    ;;
esac
`), 0o700))
	t.Setenv("GH_RECORD", recordPath)

	return helper, recordPath
}

func readInvocations(t *testing.T, recordPath string) []invocation {
	t.Helper()

	raw, err := os.ReadFile(recordPath)
	require.NoError(t, err)
	lines := bytes.Split(bytes.TrimSuffix(raw, []byte("\n")), []byte("\n"))
	invocations := make([]invocation, 0, len(lines))
	for _, line := range lines {
		fields := bytes.Split(bytes.TrimSuffix(line, []byte{0x1f}), []byte{0x1f})
		arguments := make([]string, len(fields)-1)
		for index, field := range fields[1:] {
			arguments[index] = string(field)
		}
		invocations = append(invocations, invocation{Directory: string(fields[0]), Arguments: arguments})
	}
	return invocations
}
