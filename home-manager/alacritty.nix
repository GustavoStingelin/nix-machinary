{ ... }:

{
  programs.alacritty = {
    enable = true;

    settings = {
      window = {
        opacity = 0.95;
        dynamic_title = true;
      };

      font = {
        size = 12.0;
      };

    };
  };
}
