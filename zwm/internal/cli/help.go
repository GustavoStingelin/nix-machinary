package cli

// HelpText is the complete user-facing grammar for zwm.
const HelpText = "Usage:\n" +
	"  zwm --help | -h\n" +
	"  zwm [-C <name-or-path> | --project <name-or-path>] <command>\n" +
	"\n" +
	"Commands:\n" +
	"  co <branch> | co -b <new-branch> [<start-point>]\n" +
	"  pr <number|url|branch>\n" +
	"\n" +
	"Global options:\n" +
	"  -C <name-or-path>          Select a project before the subcommand.\n" +
	"  --project <name-or-path>   Select a project before the subcommand.\n"
