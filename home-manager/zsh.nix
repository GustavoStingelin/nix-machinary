{ pkgs, ... }:
{
  # Install mise via Nix
  home.packages = with pkgs; [
    unstable.mise
    just
    bat
    ripgrep
    fzf
    nushell
    zoxide
  ];

  # Configure zsh for user
  programs.zsh = {
    enable = true;
    enableCompletion = true;
    autosuggestion.enable = true;
    syntaxHighlighting.enable = true;

    # User zsh configuration
    initContent = ''
      # Add your zsh configuration here
      export EDITOR=hx
      export VISUAL=hx
      export GIT_EDITOR=hx
      export HINDSIGHT_API_URL="http://192.168.18.174:8888"
      export PATH="$HOME/.npm-global/bin:$PATH"
      export CGO_ENABLED=0

      # Accept suggestion with Ctrl+Space
      bindkey '^ ' autosuggest-accept

      # Activate mise
      eval "$(mise activate zsh)"

      # just and zwm ship their own completions. These only need sourcing: the
      # completion system is already initialized by the time this runs, so compdef
      # is defined.
      #
      # Do NOT call compinit here. oh-my-zsh (enabled below) already runs it and
      # keeps a cached dump in ~/.zcompdump-$SHORT_HOST-$ZSH_VERSION; a second
      # compinit dumps to the default ~/.zcompdump instead and rebuilt it on every
      # single launch. That one line cost ~1.4s of a 1.7s startup — a timestamped
      # xtrace put 206,960 of 211,636 traced commands under compinit/compdump/
      # compdef. Removing it changed nothing about what is completed: 1787
      # registered completions before and after, just/zwm/git/zellij included.
      if command -v just >/dev/null 2>&1; then
        source <(just --completions zsh)
      fi

      # Enable zwm completion (branches for wco, projects for o/-C, PRs for wpr)
      if command -v zwm >/dev/null 2>&1; then
        source <(zwm completion zsh)
      fi

      # some shit bin that installs on my user folder...
      export PATH="$HOME/.signet/bin:$PATH"

      export PATH="/Users/head/.local/bin:$PATH"
    '';

    oh-my-zsh = {
      enable = true;
      theme = "robbyrussell";
      plugins = [ "git" "sudo" ];
    };

    plugins = [
      {
        name = "fzf-tab";
        src = pkgs.fetchFromGitHub {
          owner = "Aloxaf";
          repo = "fzf-tab";
          rev = "v1.2.0";
          sha256 = "sha256-Qv8zAiMtrr67CbLRrFjGaPzFZcOiMVEFLg1Z+N6VMhg=";
        };
      }
    ];
  };
}
