{ ... }:

{
  programs.neovim = {
    enable = true;
    defaultEditor = true;
    vimAlias = true;
    viAlias = true;

    withRuby = false;
    withPython3 = false;
  };

  xdg.configFile."nvim" = {
    source = ./nvim;
    recursive = true;
  };
}
