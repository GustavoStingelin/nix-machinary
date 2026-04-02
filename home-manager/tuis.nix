{ pkgs, ... }:
{
  home.packages = with pkgs; [
    k9s
    lazydocker
    tmux
  ];

  programs.lazygit = {
    enable = true;
    settings = {
      git = {
        pagers = [
          {
            colorArg = "always";
            pager = "delta --dark --paging=never";
          }
        ];
      };
    };
  };
}
