{ ... }:

{
  programs.alacritty = {
    enable = true;

    settings = {
      window = {
        opacity = 0.95;
        dynamic_title = true;
        decorations = "none";
        startup_mode = "Fullscreen";
      };

      font = {
        size = 12.0;
      };

    };
  };
}
