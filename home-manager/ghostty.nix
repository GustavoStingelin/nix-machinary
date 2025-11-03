{ ... }:

{
  programs.ghostty = {
   # enable = true;
    settings = {
      font-size = 12;
      shell-integration = "zsh";
      background-opacity = 0.95;
      background-blur-radius = 20;
      quit-after-last-window-closed = true;
    };
  };
}
