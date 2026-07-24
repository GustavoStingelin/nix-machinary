package worktree

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
)

type RegistrationState string

const (
	RegistrationAvailable  RegistrationState = "available"
	RegistrationReusable   RegistrationState = "reusable"
	RegistrationOccupied   RegistrationState = "occupied"
	RegistrationDetached   RegistrationState = "detached"
	RegistrationMismatched RegistrationState = "mismatched"
	RegistrationDuplicate  RegistrationState = "duplicate"
	RegistrationInvalid    RegistrationState = "invalid"
)

type BranchState string

const (
	BranchPrimary      BranchState = "primary"
	BranchManaged      BranchState = "managed"
	BranchUnmanaged    BranchState = "unmanaged"
	BranchUnregistered BranchState = "unregistered"
	BranchDuplicate    BranchState = "duplicate"
	BranchInvalid      BranchState = "invalid"
)

const managedWorktreeLeafLimit = 64

type TargetInput struct {
	Branch       Branch
	ManagedPath  Path
	Records      []Record
	PathOccupied bool
}

type Validation struct {
	Path         Path
	Registration RegistrationState
	Branch       BranchState
}

var ErrInvalidTarget = errors.New("invalid worktree target")

type InvalidTargetError struct {
	Validation Validation
}

func (errorValue *InvalidTargetError) Error() string {
	return "invalid worktree target"
}

func (errorValue *InvalidTargetError) Is(target error) bool {
	return target == ErrInvalidTarget
}

func ValidateTarget(input TargetInput) Validation {
	return Validation{
		Path:         input.ManagedPath,
		Registration: classifyRegistration(input),
		Branch:       classifyBranch(input),
	}
}

func (validation Validation) AcceptedPath() (Path, error) {
	switch validation.Registration {
	case RegistrationAvailable:
		if validation.Branch == BranchUnregistered {
			return validation.Path, nil
		}
	case RegistrationReusable:
		if validation.Branch == BranchManaged {
			return validation.Path, nil
		}
	}
	return "", &InvalidTargetError{Validation: validation}
}

func ManagedDisplay(identity string) string {
	var display strings.Builder
	display.Grow(len(identity))
	lastWasSeparator := false
	for index := 0; index < len(identity); index++ {
		character := identity[index]
		if (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') {
			display.WriteByte(character)
			lastWasSeparator = false
			continue
		}
		if display.Len() > 0 && !lastWasSeparator {
			display.WriteByte('-')
			lastWasSeparator = true
		}
	}
	value := strings.TrimSuffix(display.String(), "-")
	if value == "" {
		return "project"
	}
	return value
}

func IdentityHash(identity string) string {
	sum := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(sum[:])
}

func ManagedWorktreePath(root Path, identity string) Path {
	leaf := ManagedDisplay(identity)
	if len(leaf) <= managedWorktreeLeafLimit {
		return Path(filepath.Join(string(root), leaf))
	}
	suffix := IdentityHash(identity)[:8]
	croppedLength := managedWorktreeLeafLimit - len(suffix) - 1
	return Path(filepath.Join(string(root), leaf[:croppedLength]+"-"+suffix))
}

func classifyRegistration(input TargetInput) RegistrationState {
	var matching []Record
	for _, record := range input.Records {
		if record.Path == input.ManagedPath {
			matching = append(matching, record)
		}
	}
	switch len(matching) {
	case 0:
		if input.PathOccupied {
			return RegistrationOccupied
		}
		return RegistrationAvailable
	case 1:
		if matching[0].Prunable {
			return RegistrationInvalid
		}
		switch matching[0].State {
		case HeadBranch:
			if matching[0].Branch == LocalRef(input.Branch) {
				return RegistrationReusable
			}
			return RegistrationMismatched
		case HeadDetached, HeadBare:
			return RegistrationDetached
		default:
			return RegistrationInvalid
		}
	default:
		return RegistrationDuplicate
	}
}

func classifyBranch(input TargetInput) BranchState {
	matches := make([]int, 0, 1)
	pathMatches := 0
	for index, record := range input.Records {
		if record.Path == input.ManagedPath {
			pathMatches++
		}
		switch record.State {
		case HeadBranch:
			if record.Branch == LocalRef(input.Branch) {
				matches = append(matches, index)
			}
		case HeadDetached, HeadBare:
		case "":
			return BranchInvalid
		default:
			return BranchInvalid
		}
	}
	if pathMatches > 1 {
		return BranchDuplicate
	}
	switch len(matches) {
	case 0:
		return BranchUnregistered
	case 1:
		if matches[0] == 0 {
			return BranchPrimary
		}
		if input.Records[matches[0]].Path == input.ManagedPath {
			return BranchManaged
		}
		return BranchUnmanaged
	default:
		return BranchDuplicate
	}
}
