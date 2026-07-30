{ config, pkgs, ... }:
let
  # Shared signal command used by both agents' completion hooks. It is a no-op
  # outside Zellij, and otherwise pipes the caller's own pane id to the
  # zwm-attn plugin (registered in zellij.nix), which marks that pane's tab.
  zwmAttn = pkgs.writeShellScriptBin "zwm-attn" ''
    [ -n "$ZELLIJ" ] || exit 0
    exec ${pkgs.zellij}/bin/zellij pipe --plugin zwm-attn --name zwm-attn \
      --args "pane_id=''${ZELLIJ_PANE_ID},event=''${1:-finished}"
  '';

  # opencode auto-loads local plugins from ~/.config/opencode/plugins/. This
  # signals the plugin on session-idle (finished) and permission prompts
  # (waiting). opencode.json stays hand-managed.
  opencodePlugin = ''
    import type { Plugin } from "@opencode-ai/plugin"

    // Marks the Zellij tab when this opencode session finishes or needs input.
    // Delegates to the `zwm-attn` wrapper, which no-ops outside Zellij.
    export const ZwmAttnPlugin: Plugin = async ({ $ }) => ({
      event: async ({ event }) => {
        if (!process.env.ZELLIJ || !process.env.ZELLIJ_PANE_ID) return
        let signal: string | undefined
        if (event.type === "session.idle") signal = "finished"
        else if (
          event.type === "session.status" &&
          (event as any).properties?.status?.type === "idle"
        ) signal = "finished"
        else if (event.type === "permission.asked") signal = "waiting"
        if (!signal) return
        await $`zwm-attn ''${signal}`.quiet().nothrow()
      },
    })
  '';

  # Claude Code reads ~/.claude/settings.json. Home Manager owns this file, so
  # the existing hand-managed content is ported here verbatim; the Stop and
  # Notification hooks are the only additions. CLAUDE.md and settings.local.json
  # remain unmanaged.
  claudeSettings = {
    permissions = {
      allow = [ "mcp__codegraph__*" ];
      defaultMode = "auto";
    };
    hooks = {
      PreToolUse = [
        {
          matcher = "Bash";
          hooks = [ { type = "command"; command = "rtk hook claude"; } ];
        }
      ];
      UserPromptSubmit = [
        {
          hooks = [ { type = "command"; command = "codegraph prompt-hook"; } ];
        }
      ];
      # New: raise the tab attention glyph.
      Stop = [
        {
          hooks = [ { type = "command"; command = "zwm-attn finished"; } ];
        }
      ];
      Notification = [
        {
          hooks = [ { type = "command"; command = "zwm-attn waiting"; } ];
        }
      ];
    };
    worktree = { baseRef = "fresh"; };
    enabledPlugins = {
      "code-review@claude-plugins-official" = true;
      "context7@claude-plugins-official" = true;
      "github@claude-plugins-official" = true;
      "playwright@claude-plugins-official" = true;
      "feature-dev@claude-plugins-official" = true;
      "pr-review-toolkit@claude-plugins-official" = true;
      "hindsight-memory@hindsight" = true;
      "gopls-lsp@claude-plugins-official" = true;
    };
    extraKnownMarketplaces = {
      hindsight = {
        source = {
          source = "github";
          repo = "vectorize-io/hindsight";
        };
      };
    };
    askUserQuestionTimeout = "never";
    theme = "auto";
    editorMode = "normal";
  };
in
{
  home.packages = [ zwmAttn ];

  xdg.configFile."opencode/plugins/zwm-attn.ts".text = opencodePlugin;

  home.file.".claude/settings.json".text = builtins.toJSON claudeSettings;
}
