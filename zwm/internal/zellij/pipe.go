package zellij

import "context"

// The zwm-attn plugin is loaded headless in every session (see the plugins
// block in home-manager/zellij.nix) and is addressed over a CLI pipe. The
// alias, pipe name and focus event must stay byte-for-byte identical to
// PIPE_NAME and FOCUS_EVENT in zellij-plugins/zwm-attn/src/main.rs.
const (
	attnPluginAlias = "zwm-attn"
	attnPipeName    = "zwm-attn"
	focusEvent      = "focus"
)

// AttnPipe builds the CLI pipe that hands the zwm-attn plugin a pane id and an
// event: an attention state marks the pane's tab with the glyph, "focus" moves
// focus to the pane. Callers run it themselves because every use is
// best-effort and wants its own timeout.
func AttnPipe(paneID, event string) Command {
	return Command{
		Name: CommandZellij,
		Args: []string{
			"pipe", "--plugin", attnPluginAlias, "--name", attnPipeName,
			"--args", "pane_id=" + paneID + ",event=" + event,
		},
	}
}

// FocusPane focuses the terminal pane with the given $ZELLIJ_PANE_ID in the
// caller's session, switching to its tab on the way. The Zellij 0.43.1 CLI can
// focus a tab by name but not a pane by id, so this goes through the zwm-attn
// plugin, which holds the session's pane manifest and can focus a pane from
// inside the session.
func FocusPane(ctx context.Context, config Config, paneID string) (Output, error) {
	command := AttnPipe(paneID, focusEvent)
	output, err := config.Runner.Run(ctx, command)
	if err != nil {
		return output, externalFailure("focus Zellij pane", command, output, err)
	}
	return output, nil
}
