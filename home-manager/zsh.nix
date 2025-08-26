{ ... }:
{
  # Configure zsh for user
  programs.zsh = {
    enable = true;
    enableCompletion = true;
    autosuggestion.enable = true;
    syntaxHighlighting.enable = true;

    # User zsh configuration
    initContent = ''
      # Add your zsh configuration here
      export EDITOR=nvim
      export PATH="$HOME/.npm-global/bin:$PATH"
      export CGO_ENABLED=0
    '';

    oh-my-zsh = {
      enable = true;
      theme = "robbyrussell";
      plugins = [ "git" "sudo" ];
    };
  };
}
