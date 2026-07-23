package project

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
)

func canonicalExistingDirectory(directory string) (Directory, error) {
	info, err := os.Stat(directory)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("path is not a directory")
	}
	abs, err := filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return Directory(canonical), nil
}

func canonicalCodeRoot(home Directory) string {
	codeRoot := filepath.Join(string(home), "code")
	info, err := os.Stat(codeRoot)
	if err != nil || !info.IsDir() {
		return codeRoot
	}
	canonical, err := canonicalExistingDirectory(codeRoot)
	if err != nil {
		return codeRoot
	}
	return string(canonical)
}

func deriveKey(projectRoot Directory, codeRoot string) (Key, error) {
	if directCodeChild(string(projectRoot), codeRoot) {
		basename := filepath.Base(string(projectRoot))
		if !safeDirectKey(basename) {
			return "", errs.New(errs.Project, "direct project basename contains control characters and cannot be used as a project key")
		}
		return Key(basename), nil
	}

	sum := sha256.Sum256([]byte(projectRoot))
	return Key(sanitizeComponent(filepath.Base(string(projectRoot))) + "-" + hex.EncodeToString(sum[:])[:8]), nil
}

func directCodeChild(projectRoot string, codeRoot string) bool {
	relative, err := filepath.Rel(codeRoot, projectRoot)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	return !strings.Contains(relative, string(filepath.Separator))
}

func pathsOverlap(first string, second string) bool {
	return isSameOrNested(first, second) || isSameOrNested(second, first)
}

func isSameOrNested(parent string, child string) bool {
	relative, err := filepath.Rel(parent, child)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func safeDirectKey(key string) bool {
	if key == "" {
		return false
	}
	for _, value := range []byte(key) {
		if value <= 0x1f || value == 0x7f {
			return false
		}
	}
	return true
}

func sanitizeComponent(input string) string {
	var builder strings.Builder
	previousSeparator := false
	for _, value := range []byte(input) {
		if isASCIIAlphanumeric(value) {
			builder.WriteByte(value)
			previousSeparator = false
			continue
		}
		if !previousSeparator {
			builder.WriteByte('-')
			previousSeparator = true
		}
	}
	sanitized := strings.Trim(builder.String(), "-")
	if sanitized == "" {
		return "project"
	}
	return sanitized
}

func isASCIIAlphanumeric(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || value >= '0' && value <= '9'
}
