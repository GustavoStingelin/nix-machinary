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

  # opencode auto-loads local plugins from ~/.config/opencode/plugins/. Ported
  # from herdr's agent-state plugin: it maps the full session lifecycle to the
  # three attention states (working / waiting / finished), filters subagent
  # (task) sessions so their lifecycle can't clobber the root agent's state, and
  # serializes signals so the last event issued wins. opencode.json stays
  # hand-managed.
  opencodePlugin = ''
    // Marks the Zellij tab with this opencode session's attention state, via the
    // `zwm-attn` wrapper (a no-op outside Zellij).
    export const ZwmAttnPlugin = async ({ $ }) => {
      if (!process.env.ZELLIJ || !process.env.ZELLIJ_PANE_ID) return {}

      // Serialize signals so the last event issued is the last one written.
      let chain = Promise.resolve()
      const signal = (state) => {
        chain = chain
          .then(() => $`zwm-attn ''${state} --agent opencode`.quiet().nothrow())
          .catch(() => {})
        return chain
      }

      // Subagent (task) sessions carry a parentID; drop their lifecycle events so
      // they can't clobber the pane's root state, but still surface one that is
      // blocked on the user.
      const childSessions = new Set()

      const stateFromStatus = (status) => {
        const kind = typeof status === "string" ? status : status?.type
        if (typeof kind !== "string") return undefined
        switch (kind.toLowerCase()) {
          case "idle":
            return "finished"
          case "active":
          case "busy":
          case "pending":
          case "running":
          case "streaming":
          case "working":
          case "retry":
            return "working"
          default:
            return undefined
        }
      }

      return {
        "chat.message": async ({ sessionID }) => {
          if (sessionID && childSessions.has(sessionID)) return
          await signal("working")
        },
        event: async ({ event }) => {
          const type = event?.type
          const properties = event?.properties ?? {}
          const sessionID =
            typeof properties.sessionID === "string" ? properties.sessionID : undefined

          const info = properties.info
          if (info?.id && info.parentID) childSessions.add(info.id)

          if (sessionID && childSessions.has(sessionID)) {
            switch (type) {
              case "permission.asked":
              case "question.asked":
                await signal("waiting")
                break
              case "permission.replied":
              case "question.replied":
              case "question.rejected":
                await signal("working")
                break
            }
            return
          }

          switch (type) {
            case "session.status": {
              const state = stateFromStatus(properties.status)
              if (state) await signal(state)
              break
            }
            case "tool.execute.before":
            case "tool.execute.after":
            case "permission.replied":
            case "question.replied":
            case "question.rejected":
            case "session.compacted":
              await signal("working")
              break
            case "permission.asked":
            case "question.asked":
            case "session.error":
              await signal("waiting")
              break
            case "session.idle":
              await signal("finished")
              break
          }
        },
      }
    }
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
