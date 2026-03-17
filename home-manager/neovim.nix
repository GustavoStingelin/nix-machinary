{ ... }:

{
  programs.neovim = {
    enable = true;
    defaultEditor = true;
    vimAlias = true;
    viAlias = true;
  };

  xdg.configFile."nvim" = {
    source = ./nvim;
    recursive = true;
  };
}
