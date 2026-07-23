package worktree

import (
	"bytes"
	"errors"
	"strings"
)

var ErrMalformedPorcelain = errors.New("malformed git worktree porcelain")

type ParseError struct {
	Reason string
}

func (errorValue *ParseError) Error() string {
	return "malformed git worktree porcelain: " + errorValue.Reason
}

func (errorValue *ParseError) Is(target error) bool {
	return target == ErrMalformedPorcelain
}

func ParsePorcelainZ(raw []byte) ([]Record, error) {
	if len(raw) == 0 {
		return nil, malformed("empty output")
	}
	if raw[len(raw)-1] != 0 {
		return nil, malformed("missing final NUL")
	}

	fields := bytes.Split(raw[:len(raw)-1], []byte{0})
	records := make([]Record, 0, len(fields)/4)
	builder := recordBuilder{}
	for _, field := range fields {
		if len(field) == 0 {
			record, err := builder.finish()
			if err != nil {
				return nil, err
			}
			records = append(records, record)
			builder = recordBuilder{}
			continue
		}
		if bytes.HasPrefix(field, []byte("worktree ")) {
			if builder.active {
				return nil, malformed("record missing separator")
			}
			if err := builder.start(field); err != nil {
				return nil, err
			}
			continue
		}
		if err := builder.add(field); err != nil {
			return nil, err
		}
	}
	if builder.active {
		return nil, malformed("truncated record")
	}
	if len(records) == 0 {
		return nil, malformed("empty output")
	}
	return records, nil
}

type recordBuilder struct {
	active      bool
	path        Path
	head        OID
	branch      Ref
	state       HeadState
	headSet     bool
	lockedSet   bool
	locked      string
	prunableSet bool
	prunable    string
}

func (builder *recordBuilder) start(field []byte) error {
	_, value, found := bytes.Cut(field, []byte(" "))
	if !found || len(value) == 0 {
		return malformed("missing worktree path")
	}
	builder.active = true
	builder.path = Path(string(value))
	return nil
}

func (builder *recordBuilder) add(field []byte) error {
	if !builder.active {
		return malformed("attribute before worktree")
	}
	label, value, hasValue := bytes.Cut(field, []byte(" "))
	switch string(label) {
	case "HEAD":
		if !hasValue || builder.headSet || !validOID(value) {
			return malformed("invalid HEAD")
		}
		builder.headSet = true
		builder.head = OID(string(value))
	case "branch":
		if !hasValue || builder.state != "" || !validLocalRef(value) {
			return malformed("invalid branch")
		}
		builder.state = HeadBranch
		builder.branch = Ref(string(value))
	case "detached":
		if hasValue || builder.state != "" {
			return malformed("invalid detached attribute")
		}
		builder.state = HeadDetached
	case "bare":
		if hasValue || builder.state != "" {
			return malformed("invalid bare attribute")
		}
		builder.state = HeadBare
	case "locked":
		if builder.lockedSet {
			return malformed("duplicate locked attribute")
		}
		builder.lockedSet = true
		if hasValue {
			builder.locked = string(value)
		}
	case "prunable":
		if builder.prunableSet {
			return malformed("duplicate prunable attribute")
		}
		builder.prunableSet = true
		if hasValue {
			builder.prunable = string(value)
		}
	default:
		return malformed("unknown attribute " + string(label))
	}
	return nil
}

func (builder recordBuilder) finish() (Record, error) {
	if !builder.active {
		return Record{}, malformed("empty record")
	}
	switch builder.state {
	case HeadBare:
		if builder.headSet {
			return Record{}, malformed("bare record has HEAD")
		}
	case HeadBranch, HeadDetached:
		if !builder.headSet {
			return Record{}, malformed("record missing HEAD")
		}
	default:
		return Record{}, malformed("record missing checkout state")
	}
	return Record{
		Path:           builder.path,
		Head:           builder.head,
		Branch:         builder.branch,
		State:          builder.state,
		Locked:         builder.locked,
		Prunable:       builder.prunableSet,
		PrunableReason: builder.prunable,
	}, nil
}

func malformed(reason string) error {
	return &ParseError{Reason: reason}
}

func validOID(raw []byte) bool {
	if len(raw) != 40 && len(raw) != 64 {
		return false
	}
	for _, character := range raw {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') && (character < 'A' || character > 'F') {
			return false
		}
	}
	return true
}

func validLocalRef(raw []byte) bool {
	const prefix = "refs/heads/"
	if !bytes.HasPrefix(raw, []byte(prefix)) || len(raw) == len(prefix) {
		return false
	}
	name := string(raw[len(prefix):])
	if strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") || strings.Contains(name, "//") || strings.Contains(name, "..") || strings.Contains(name, "@{") {
		return false
	}
	for component := range strings.SplitSeq(name, "/") {
		if strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") || strings.HasSuffix(component, ".") {
			return false
		}
		for _, character := range component {
			if character <= 0x20 || character == 0x7f || strings.ContainsRune("~^:?*[\\", character) {
				return false
			}
		}
	}
	return true
}
