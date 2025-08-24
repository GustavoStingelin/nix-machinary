{ ... }:
{
  programs.git = {
    enable = true;

    # User git configuration
    userName = "Gustavo Stingelin";
    userEmail = "gustavo.stingelin@outlook.com";

    extraConfig = {
      init.defaultBranch = "main";

      # GPG signing configuration
      user.signingkey = "0x15CBADFE29F2017B";
      commit.gpgsign = true;
      tag.gpgsign = true;
      gpg.program = "gpg";

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
