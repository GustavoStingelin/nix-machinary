{ pkgs, ... }:
{
  # Install Ruby with Jekyll (bundler comes with Ruby)
  home.packages = with pkgs; [
    ruby_3_4
    rubyPackages_3_4.jekyll
  ];

  # Set up Ruby environment
  home.sessionVariables = {
    GEM_HOME = "$HOME/.gem";
    GEM_PATH = "$HOME/.gem";
  };

  home.sessionPath = [
    "$HOME/.gem/bin"
  ];
}
