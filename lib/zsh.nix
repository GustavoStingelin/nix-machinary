{ config, pkgs, lib, ... }:
{
  # Configure zsh system-wide
  programs.zsh = {
    enable = true;
    enableCompletion = true;
    autosuggestions.enable = true;
    syntaxHighlighting.enable = true;

    # System-wide zsh configuration
    shellInit = ''
      # Add your zsh configuration here
      export EDITOR=nvim
    '';

    ohMyZsh = {
      enable = true;
      theme = "robbyrussell";
      plugins = [ "git" "sudo" ];
    };
  };

  # Set zsh as default shell for your user
  users.users.head.shell = pkgs.zsh;
}