{ config, pkgs, ... }:
let
  # Shared signal command used by both agents' completion hooks. It forwards to
  # `zwm attn`, which records the three-way attention state (working/waiting/done)
  # for this pane and raises the zwm-attn tab glyph. `zwm attn` is itself a no-op
  # outside Zellij, so the wrapper stays a thin passthrough. Kept as `zwm-attn`
  # for the opencode plugin, which invokes it by name.
  zwmAttn = pkgs.writeShellScriptBin "zwm-attn" ''
    exec ${pkgs.zwm}/bin/zwm attn "$@"
  '';

  # opencode auto-loads local plugins from ~/.config/opencode/plugins/. This
  # signals the three attention states: busy (working), permission prompts
  # (waiting), and session-idle (finished/done). opencode.json stays hand-managed.
  opencodePlugin = ''
    import type { Plugin } from "@opencode-ai/plugin"

    // Marks the Zellij tab with this opencode session's attention state.
    // Delegates to the `zwm-attn` wrapper, which no-ops outside Zellij.
    export const ZwmAttnPlugin: Plugin = async ({ $ }) => ({
      event: async ({ event }) => {
        if (!process.env.ZELLIJ || !process.env.ZELLIJ_PANE_ID) return
        let signal: string | undefined
        if (event.type === "session.idle") signal = "finished"
        else if (event.type === "session.status") {
          const status = (event as any).properties?.status?.type
          signal = status === "idle" ? "finished" : "working"
        } else if (event.type === "permission.asked") signal = "waiting"
        if (!signal) return
        await $`zwm-attn ''${signal} --agent opencode`.quiet().nothrow()
      },
    })
  '';

  # Claude Code reads ~/.claude/settings.json. Home Manager owns this file, so
  # the existing hand-managed content is ported here verbatim; the zwm-attn
  # attention hooks (UserPromptSubmit=working, Notification=waiting, Stop=done)
  # are the only additions. CLAUDE.md and settings.local.json remain unmanaged.
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
          hooks = [
            { type = "command"; command = "codegraph prompt-hook"; }
            { type = "command"; command = "zwm-attn working --agent claude"; }
          ];
        }
      ];
      # Raise the tab attention glyph and record the three-way state.
      Stop = [
        {
          hooks = [ { type = "command"; command = "zwm-attn done --agent claude"; } ];
        }
      ];
      Notification = [
        {
          hooks = [ { type = "command"; command = "zwm-attn waiting --agent claude"; } ];
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
