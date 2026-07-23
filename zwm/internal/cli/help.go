package cli

// HelpText is the complete user-facing grammar for zwm.
const HelpText = "Usage:\n" +
	"  zwm --help | -h\n" +
	"  zwm [-C <name-or-path> | --project <name-or-path>] {wco|pr}\n" +
	"  zwm o <name-or-path>\n" +
	"\n" +
	"Commands:\n" +
	"  wco <branch> | wco -b <new-branch> [<start-point>]\n" +
	"  o <name-or-path>\n" +
	"  pr <number|url|branch>\n" +
	"\n" +
	"Global options:\n" +
	"  -C <name-or-path>          Select a project before the subcommand.\n" +
	"  --project <name-or-path>   Select a project before the subcommand.\n"
