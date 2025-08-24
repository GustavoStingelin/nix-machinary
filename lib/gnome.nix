{ config, pkgs, lib, ... }:
{
  # Enable GNOME Desktop Environment
  services.xserver = {
    enable = true;
    displayManager.gdm.enable = true;
    desktopManager.gnome.enable = true;
  };
  # Remove default GNOME applications
  environment.gnome.excludePackages = with pkgs; [
    gnome-photos
    gnome-tour
    gnome-music
    gnome-maps
    gnome-contacts
    gnome-weather
    epiphany # GNOME Web browser
    geary # GNOME Mail
    evince # GNOME Document viewer
    totem # GNOME Videos
    tali # GNOME Poker game
    iagno # GNOME Reversi game
    hitori # GNOME Sudoku game
    atomix # GNOME Puzzle game
    gnome-klotski
    gnome-mines
    gnome-nibbles
    gnome-robots
    gnome-sudoku
    gnome-taquin
    gnome-tetravex
    quadrapassel
    swell-foop
    lightsoff
    four-in-a-row
    gnome-chess
    yelp # GNOME Help
    gnome-logs
    #baobab # GNOME Disk Usage Analyzer
    gnome-font-viewer
    gnome-screenshot
    #simple-scan
    #gnome-system-monitor
    seahorse # GNOME Passwords and Keys
    #gnome-disk-utility
  ];
  # Install required packages for extensions and GNOME functionality
  environment.systemPackages = with pkgs; [
    # Extension requirements
    glib # Required for GSettings and various GNOME extensions
    gtk3 # GTK3 libraries
    gtk4 # GTK4 libraries for newer extensions
    libgtop # System monitoring library (required for Vitals)
    lm_sensors # Hardware sensors (required for Vitals)
    hddtemp # Hard disk temperature (for Vitals)
    udisks # Disk management (for Removable Drive Menu)
    upower # Power management info (for Vitals)
    networkmanager # Network management (for Vitals network stats)
    # GNOME Shell extensions
    gnomeExtensions.blur-my-shell
    gnomeExtensions.caffeine
    gnomeExtensions.dash-to-dock
    #gnomeExtensions.gamemode # Note: GameBar might need this
    gnomeExtensions.removable-drive-menu
    gnomeExtensions.tiling-assistant
    gnomeExtensions.vitals
    gnomeExtensions.status-icons
    
    # Additional useful tools
    gnome-tweaks # For managing extensions
    dconf-editor # For advanced GNOME settings
  ];
  # Programs that complement the setup
  programs = {
    # Gaming mode support (for GameBar overlay)
    gamemode.enable = true;
  };
}
