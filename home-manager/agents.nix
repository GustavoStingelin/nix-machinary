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

  # opencode v2 runs one shared background server (`opencode serve --service`)
  # whose env belongs to whichever pane started it, so a server plugin can't
  # tell panes apart. CLI plugins run inside each pane's own `opencode` process
  # instead, so this is one: opencode discovers <config>/plugins/<name>/tui.ts
  # as a CLI plugin, and with no index.ts beside it the server ignores it. It maps the session lifecycle to the three attention
  # states (working / waiting / finished), skips subagent sessions so they can't
  # clobber the root agent's state, and serializes signals so the last wins.
  opencodePlugin = ''
    import { execFile, execFileSync } from "node:child_process"

    const run = (args) =>
      new Promise((resolve) => {
        execFile("zwm-attn", args, { timeout: 2000 }, () => resolve())
      })

    // Marks the Zellij tab with this opencode session's attention state, via the
    // `zwm-attn` wrapper (a no-op outside Zellij).
    export default {
      id: "zwm-attn",
      setup(ctx) {
        if (!process.env.ZELLIJ || !process.env.ZELLIJ_PANE_ID) return

        // When opencode exits (its tab/pane may stay open), forget the record so
        // the dashboard doesn't keep showing a closed agent. Synchronous so it
        // completes during process teardown.
        process.once("exit", () => {
          try {
            execFileSync("zwm-attn", ["closed", "--agent", "opencode"], {
              stdio: "ignore",
              timeout: 2000,
            })
          } catch {}
        })

        // Serialize signals so the last event issued is the last one written.
        let chain = Promise.resolve()
        const signal = (state) => {
          chain = chain.then(() => run([state, "--agent", "opencode"])).catch(() => {})
          return chain
        }

        // Subagent (task) sessions have a root other than themselves; drop their
        // lifecycle so it can't clobber the pane's state, but still surface one
        // that is blocked on the user.
        const isChild = async (sessionID) => {
          if (!sessionID) return false
          try {
            const root = await ctx.data.session.root(sessionID)
            const rootID = typeof root === "string" ? root : root?.id
            return Boolean(rootID) && rootID !== sessionID
          } catch {
            return false
          }
        }

        const on = (type, state, alsoForChildren = false) =>
          ctx.data.on(type, async (event) => {
            const sessionID = event?.data?.sessionID ?? event?.sessionID
            if (!alsoForChildren && (await isChild(sessionID))) return
            await signal(state)
          })

        const unsubscribe = [
          on("session.execution.started", "working"),
          on("session.execution.succeeded", "finished"),
          on("session.execution.interrupted", "finished"),
          on("session.execution.failed", "waiting"),
          on("session.error", "waiting"),
          on("permission.asked", "waiting", true),
          on("permission.replied", "working", true),
          on("permission.rejected", "working", true),
        ]
        return () => unsubscribe.forEach((off) => off?.())
      },
    }
  '';

  # Hindsight coding-agents runtime, staged imperatively by
  # `npx @vectorize-io/hindsight-coding-agents install claude-code`. Its
  # installer cannot write the store-backed settings.json, so the hooks it
  # would merge are declared here instead.
  hindsightRuntime = "${config.home.homeDirectory}/.hindsight/coding-agents";
  hindsightHook = file: timeout: {
    type = "command";
    command = ''node "${hindsightRuntime}/dist/${file}"'';
    inherit timeout;
  };

  # Claude Code reads ~/.claude/settings.json. Home Manager owns this file, so
  # the existing hand-managed content is ported here verbatim. Additions are
  # the zwm-attn attention hooks (UserPromptSubmit=working, Notification=waiting,
  # Stop=done) and the Hindsight integration: its hooks, MCP permissions,
  # longer transcript retention, and disabled auto memory. CLAUDE.md and
  # settings.local.json remain unmanaged.
  claudeSettings = {
    # `hindsight-banks` is the Hindsight server's own MCP endpoint, where every
    # tool takes a bank_id, so an agent can reach any bank rather than only the
    # current repo's. It lives in the mutable ~/.claude.json, registered with
    # `claude mcp add --scope user --transport http hindsight-banks
    # http://192.168.18.174:8888/mcp/`. Reads are allowed, writes prompt, and
    # the destructive tools are denied outright.
    permissions = {
      allow = [
        "mcp__codegraph__*"
      ]
      ++ map (tool: "mcp__hindsight-banks__${tool}") [
        "list_banks"
        "get_bank"
        "get_bank_stats"
        "recall"
        "reflect"
        "list_memories"
        "get_memory"
        "list_documents"
        "get_document"
        "list_tags"
        "list_operations"
        "get_operation"
        "list_mental_models"
        "get_mental_model"
        "list_directives"
        "get_knowledge_base_tree"
        "search_knowledge_base"
        "get_knowledge_page"
      ];
      deny = map (tool: "mcp__hindsight-banks__${tool}") [
        "delete_bank"
        "clear_memories"
        "delete_document"
        "delete_knowledge_node"
        "delete_mental_model"
        "clear_mental_model"
        "delete_directive"
        "invalidate_memory"
        "cancel_operation"
        "update_bank"
      ];
      defaultMode = "auto";
    };
    # Keep transcripts well past the 30-day default so past sessions can still
    # be imported into their repo's Hindsight bank.
    cleanupPeriodDays = 365;
    hooks = {
      PreToolUse = [
        {
          matcher = "Bash";
          hooks = [ { type = "command"; command = "rtk hook claude"; } ];
        }
      ];
      # Hindsight: session-start context, per-prompt recall, retain on stop.
      SessionStart = [
        { hooks = [ (hindsightHook "claude-sessionstart-hook.js" 30) ]; }
      ];
      UserPromptSubmit = [
        {
          hooks = [
            { type = "command"; command = "codegraph prompt-hook"; }
            { type = "command"; command = "zwm-attn working --agent claude"; }
            (hindsightHook "claude-hook.js" 30)
          ];
        }
      ];
      # Raise the tab attention glyph and record the three-way state.
      Stop = [
        {
          hooks = [
            { type = "command"; command = "zwm-attn done --agent claude"; }
            (hindsightHook "claude-stop-hook.js" 60)
          ];
        }
      ];
      Notification = [
        {
          hooks = [ { type = "command"; command = "zwm-attn waiting --agent claude"; } ];
        }
      ];
      # When the session ends (Claude exits), forget the record so the dashboard
      # doesn't keep showing a closed agent while its tab stays open.
      SessionEnd = [
        {
          hooks = [ { type = "command"; command = "zwm-attn closed --agent claude"; } ];
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
      # Superseded by the coding-agents runtime; both on would double every
      # recall and retain, and their MCP servers share the `hindsight` name.
      "hindsight-memory@hindsight" = false;
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
    # Hindsight is the single long-term memory; Claude Code's file-based auto
    # memory would split knowledge into a store opencode can't see.
    autoMemoryEnabled = false;
    # No Co-Authored-By trailer on commits or "Generated with Claude Code" line
    # on PRs. Object form, since older Claude Code rejects `attribution = false`.
    attribution = {
      commit = "";
      pr = "";
      sessionUrl = false;
    };
  };

  # Coding-agents runtime config (~/.hindsight/coding-agent.json). No bankId,
  # so each repo resolves to its own `coding-agent::<repo>` bank: one shared
  # bank let whichever repo had the most history crowd every other repo out of
  # the knowledge pages and per-prompt recall. The old shared `opencode` bank
  # stays on the server as a cross-repo archive, reachable through the
  # `hindsight-banks` MCP server. The runtime sets each bank's missions itself.
  # autoUpdate stays off so the runtime changes only when this repo says so.
  hindsightCodingAgentConfig = {
    serverMode = "self-hosted";
    apiUrl = "http://192.168.18.174:8888";
    autoUpdate = false;
  };
in
{
  home.packages = [ zwmAttn ];

  xdg.configFile."opencode/plugins/zwm-attn/tui.ts".text = opencodePlugin;

  home.file.".claude/settings.json".text = builtins.toJSON claudeSettings;

  home.file.".hindsight/coding-agent.json".text =
    builtins.toJSON hindsightCodingAgentConfig;

  home.file.".claude/skills/hindsight-coding-agent".source =
    config.lib.file.mkOutOfStoreSymlink "${hindsightRuntime}/skill";

  # OMO (omo-ai) is a pi fork whose agent dir is ~/.omo/agent, so it never sees
  # the runtime's pi extension registered in ~/.pi/agent/settings.json. It
  # auto-loads every global extension in ~/.omo/agent/extensions/, so re-export
  # the same pi extension there, the way OMO's own built-ins are shimmed.
  home.file.".omo/agent/extensions/hindsight.js".text = ''
    export { default } from "file://${hindsightRuntime}/dist/pi.js";
  '';
}
