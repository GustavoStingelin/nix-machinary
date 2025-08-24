{ pkgs, ... }:

{
  home.packages = with pkgs; [
    # Language servers
    nil
    nixd                  # Nix LSP
    rust-analyzer         # Rust LSP
    gopls                 # Go LSP
    pyright               # Python LSP
    typescript-language-server  # TypeScript/JavaScript LSP
    vscode-langservers-extracted  # HTML/CSS/JSON/ESLint LSPs
    lua-language-server   # Lua LSP
    clang-tools          # C/C++ LSP (clangd)
    zls                  # Zig LSP
    marksman             # Markdown LSP
    yaml-language-server # YAML LSP
    taplo                # TOML LSP
    bash-language-server # Bash LSP
    dockerfile-language-server-nodejs  # Dockerfile LSP
    terraform-ls         # Terraform LSP

    # Formatters
    nixpkgs-fmt          # Nix formatter
    rustfmt              # Rust formatter
    gofumpt              # Go formatter
    black                # Python formatter
    nodePackages.prettier # JS/TS/HTML/CSS formatter
    stylua               # Lua formatter
    shfmt                # Shell script formatter

    # Linters
    statix               # Nix linter
    clippy               # Rust linter
    golangci-lint        # Go linter
    ruff                 # Python linter/formatter
    nodePackages.eslint  # JavaScript/TypeScript linter
    shellcheck           # Shell script linter
    typos                # Spell checker
    typos-lsp            # Typos LSP server

    # Debug adapters
    delve                # Go debugger
    lldb                 # C/C++/Rust debugger
  ];
}
