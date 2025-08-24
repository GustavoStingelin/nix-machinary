{ config, lib, pkgs, ... }:

{
  programs.git = {
    enable = true;
    
    # System-wide git configuration
    config = {
      init.defaultBranch = "main";
      user.name = "Gustavo Stingelin";
      user.email = "gustavo.stingelin@outlook.com";
      
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
