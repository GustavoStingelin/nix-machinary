package git

type Directory string

type Branch string

type Commitish string

type Commit string

type WorktreePath string

type Config struct {
	Executable string
}

type Client struct {
	executable string
}

func NewClient(config Config) Client {
	executable := config.Executable
	if executable == "" {
		executable = "git"
	}
	return Client{executable: executable}
}
