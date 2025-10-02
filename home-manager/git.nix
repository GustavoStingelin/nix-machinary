{ ... }:
{
  programs.git = {
    enable = true;

    # User git configuration
    userName = "Gustavo Stingelin";
    userEmail = "gustavo.stingelin@outlook.com";

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
    ];

    extraConfig = {
      init.defaultBranch = "main";

      # GPG signing configuration
      user.signingkey = "0x15CBADFE29F2017B";
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
      };
    };
  };

  programs.gh = {
    enable = true;
    settings = {
      git_protocol = "ssh";
    };
  };
}
