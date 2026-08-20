{ ... }:
{
  programs.git = {
    enable = true;

    ignores = [
      "debug/"
      "target/"

      # for projects that don't use mise...
      ".mise/"

      # Mac
      ".DS_Store"
      ".fseventsd"
      ".Spotlight-V100"
      ".TemporaryItems"
      ".Trashes"

      # Helix
      ".helix/"

      # Zed
      ".zed/"

      # VSCode Workspace Folder
      ".vscode/"

      #Jetbrains
      ".idea/"

      # Golang
      ".gocache/"
      ".gomodcache/"

      # Python
      "*.pyc"
      "*.egg"
      "*.out"
      "venv/"
      "**/**/__pycache__/"

      # direnv
      ".direnv"
      ".envrc"

      # NodeJS/Web dev
      ".env/"
      "node_modules"
      ".sass-cache"

      # Claude
      "**/.claude/settings.local.json"

      #ignore my own just commands, bc some projects only uses make...
      ".justfile"

      #ignore as my own instructions
      "AGENTS.md"
      ".sisyphus/"
      ".omo/"
      ".codegraph/"
      
    ];

    settings = {
      # User git configuration
      user = {
        name = "Gustavo Stingelin";
        email = "gustavo.stingelin@outlook.com";

        # GPG signing configuration
        signingkey = "0x15CBADFE29F2017B";
      };

      init.defaultBranch = "main";
      core.editor = "hx";

      # Signature status in the log. `%G?` is a single letter: G good, U good but
      # the key is not ultimately trusted, N unsigned, B bad, E unverifiable.
      # `sig` replaces the default `medium` format, `sigline` is the --oneline
      # equivalent behind the `sl` alias.
      pretty = {
        sig = "format:%C(auto)commit %H%d%C(reset)%nSignature: %C(auto)%G?%C(reset) %GS%nAuthor:    %an <%ae>%nDate:      %ad%n%n%w(0,4,4)%B";
        sigline = "format:%C(auto)%h %G?%d%C(reset) %s %C(dim)(%ar)%C(reset)";
      };
      format.pretty = "sig";

      # Delta pager options (core.pager and interactive.diffFilter are set automatically by programs.delta)
      merge.conflictstyle = "diff3";
      diff.colorMoved = "default";

      commit.gpgsign = true;
      tag.gpgsign = true;
      gpg.program = "gpg";

      push.autoSetupRemote = true;

      # Git aliases
      alias = {
        a = "add";
        cm = "commit";
        co = "checkout";
        st = "status";
        last = "log -1 HEAD";
        sl = "log --pretty=sigline";
      };
    };

  };

  # Home Manager 26.05 moved delta out from under programs.git, and no longer
  # wires it into git implicitly just because it is enabled.
  programs.delta = {
    enable = true;
    enableGitIntegration = true;
    options = {
      navigate = true;
      side-by-side = true;
      line-numbers = true;
      syntax-theme = "ansi";
    };
  };
}
