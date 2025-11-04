{ ... }:

{
  programs.alacritty = {
    enable = true;

    settings = {
      window = {
        opacity = 0.95;
        dynamic_title = true;
        decorations = "none";
        startup_mode = "Maximized";
      };

      font = {
        size = 12.0;
      };

    };
  };
}
