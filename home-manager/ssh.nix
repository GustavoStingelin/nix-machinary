{ ... }:
{
  programs.ssh = {
    enable = true;

    # OrbStack requires its Include before any Host block; `includes` is emitted
    # in the global section, ahead of every block.
    includes = [ "~/.orbstack/ssh/config" ];

    enableDefaultConfig = false;

    settings = {
      "github.com" = {
        User = "git";
        IdentityFile = "~/.ssh/github_gustavostingelin";
        IdentitiesOnly = true;
      };
    };
  };
}
