{ ... }:
{
  programs.atuin = {
    enable = true;
    enableZshIntegration = true;
    
    settings = {
      # Use a more compact view
      style = "compact";
      
      # Show time stamps
      show_preview = true;
      
      # Update frequency (in seconds)
      update_check = false;
      
      # Sync configuration (disabled by default)
      sync_frequency = "0";
      
      # Search mode
      search_mode = "fuzzy";
      
      # Filter mode
      filter_mode = "global";
      
      # Enter accepts suggestion
      enter_accept = true;
      
      # History file settings
      history_filter = [
        "^ls$"
        "^cd$"
        "^pwd$"
        "^exit$"
        "^clear$"
      ];
    };
  };
}