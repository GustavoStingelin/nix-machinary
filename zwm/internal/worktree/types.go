package worktree

type Path string

type OID string

type Ref string

type Branch string

type HeadState string

const (
	HeadBranch   HeadState = "branch"
	HeadDetached HeadState = "detached"
	HeadBare     HeadState = "bare"
)

type Record struct {
	Path           Path
	Head           OID
	Branch         Ref
	State          HeadState
	Locked         string
	Prunable       bool
	PrunableReason string
}

func LocalRef(branch Branch) Ref {
	return Ref("refs/heads/" + string(branch))
}
