{ ... }:

{
  # Set Vivaldi as default browser using XDG
  xdg.mimeApps = {
    enable = true;
    defaultApplications = {
      "text/html" = "vivaldi-stable.desktop";
      "x-scheme-handler/http" = "vivaldi-stable.desktop";
      "x-scheme-handler/https" = "vivaldi-stable.desktop";
      "x-scheme-handler/about" = "vivaldi-stable.desktop";
      "x-scheme-handler/unknown" = "vivaldi-stable.desktop";
    };
  };

  dconf = {
    enable = true;
    settings = {
      # GNOME Dark mode
      "org/gnome/desktop/interface" = {
        color-scheme = "prefer-dark";
        gtk-theme = "Adwaita-dark";
      };
      # Better video display
      "org/gnome/mutter" = {
        experimental-features = [
          "scale-monitor-framebuffer" # Enables fractional scaling (125% 150% 175%)
          "variable-refresh-rate" # Enables Variable Refresh Rate (VRR) on compatible displays
          "xwayland-native-scaling" # Scales Xwayland applications to look crisp on HiDPI screens
        ];
      };
      # Custom keybinding for Ghostty
      "org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/custom0" = {
        name = "Open Ghostty";
        command = "nvidia-offload ghostty";
        binding = "<Super>t";
      };
      # Custom keybinding for GoLand
      "org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/custom1" = {
        name = "Open GoLand";
        command = "nvidia-offload goland";
        binding = "<Super>g";
      };
      # Custom keybinding for ChatGPT
      "org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/custom2" = {
        name = "Open ChatGPT";
        command = "nvidia-offload vivaldi https://chatgpt.com";
        binding = "<Super>c";
      };
      # Custom keybinding for X.com
      "org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/custom3" = {
        name = "Open X.com";
        command = "nvidia-offload vivaldi https://x.com";
        binding = "<Super>x";
      };
      # Custom keybinding for Zed Editor
      "org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/custom4" = {
        name = "Open Zed Editor";
        command = "nvidia-offload zeditor";
        binding = "<Super>z";
      };
      # Custom keybinding for Vivaldi Browser
      "org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/custom5" = {
        name = "Open Vivaldi Browser";
        command = "nvidia-offload vivaldi";
        binding = "<Super>b";
      };
      "org/gnome/settings-daemon/plugins/media-keys" = {
        custom-keybindings = [
          "/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/custom0/"
          "/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/custom1/"
          "/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/custom2/"
          "/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/custom3/"
          "/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/custom4/"
          "/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/custom5/"
        ];
      };
    };
  };
}
