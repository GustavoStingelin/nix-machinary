{ ... }:
{
  programs.git = {
    enable = true;

    # User git configuration
    userName = "Gustavo Stingelin";
    userEmail = "gustavo.stingelin@outlook.com";

    extraConfig = {
      init.defaultBranch = "main";

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
}
