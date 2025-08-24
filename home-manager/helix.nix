{ ... }:

{
  programs.helix = {
    enable = true;
    settings = {
      theme = "catppuccin_mocha";

      editor = {
        line-number = "relative";
        mouse = false;
        cursor-shape = {
          insert = "bar";
          normal = "block";
          select = "underline";
        };
        indent-guides.render = true;
        color-modes = true;
        auto-format = true;
        auto-save = true;
        completion-trigger-len = 1;
      };

      keys.normal = {
        space.space = "file_picker";
        space.w = ":w";
        space.q = ":q";
        esc = [ "collapse_selection" "keep_primary_selection" ];
      };
    };

    languages = {
      language-server.nil = {
        command = "nil";
        config.nil.formatting.command = [ "nixpkgs-fmt" ];
      };

      language-server.rust-analyzer = {
        command = "rust-analyzer";
        config.rust-analyzer = {
          cargo.loadOutDirsFromCheck = true;
          procMacro.enable = true;
          checkOnSave.command = "clippy";
        };
      };

      language-server.gopls = {
        command = "gopls";
        config.gopls = {
          gofumpt = true;
          staticcheck = true;
          usePlaceholders = true;
        };
      };

      language-server.pyright = {
        command = "pyright-langserver";
        args = [ "--stdio" ];
      };

      language-server.typescript-language-server = {
        command = "typescript-language-server";
        args = [ "--stdio" ];
      };

      language-server.vscode-html-language-server = {
        command = "vscode-html-language-server";
        args = [ "--stdio" ];
      };

      language-server.vscode-css-language-server = {
        command = "vscode-css-language-server";
        args = [ "--stdio" ];
      };

      language-server.vscode-json-language-server = {
        command = "vscode-json-language-server";
        args = [ "--stdio" ];
      };

      language-server.lua-language-server = {
        command = "lua-language-server";
      };

      language-server.clangd = {
        command = "clangd";
      };

      language-server.zls = {
        command = "zls";
      };

      language-server.marksman = {
        command = "marksman";
        args = [ "server" ];
      };

      language-server.yaml-language-server = {
        command = "yaml-language-server";
        args = [ "--stdio" ];
      };

      language-server.taplo = {
        command = "taplo";
        args = [ "lsp" "stdio" ];
      };

      language-server.bash-language-server = {
        command = "bash-language-server";
        args = [ "start" ];
      };

      language-server.dockerfile-language-server = {
        command = "docker-langserver";
        args = [ "--stdio" ];
      };

      language-server.terraform-ls = {
        command = "terraform-ls";
        args = [ "serve" ];
      };

      language = [
        {
          name = "nix";
          scope = "source.nix";
          file-types = [ "nix" ];
          language-servers = [ "nil" ];
          formatter.command = "nixpkgs-fmt";
        }
        {
          name = "rust";
          scope = "source.rust";
          file-types = [ "rs" ];
          language-servers = [ "rust-analyzer" ];
          formatter.command = "rustfmt";
        }
        {
          name = "go";
          scope = "source.go";
          file-types = [ "go" ];
          language-servers = [ "gopls" ];
          formatter.command = "gofumpt";
        }
        {
          name = "python";
          scope = "source.python";
          file-types = [ "py" "pyi" ];
          language-servers = [ "pyright" ];
          formatter.command = "black";
          formatter.args = [ "-" ];
        }
        {
          name = "typescript";
          scope = "source.ts";
          file-types = [ "ts" "tsx" ];
          language-servers = [ "typescript-language-server" ];
          formatter.command = "prettier";
          formatter.args = [ "--parser" "typescript" ];
        }
        {
          name = "javascript";
          scope = "source.js";
          file-types = [ "js" "jsx" ];
          language-servers = [ "typescript-language-server" ];
          formatter.command = "prettier";
          formatter.args = [ "--parser" "javascript" ];
        }
        {
          name = "html";
          scope = "text.html.basic";
          file-types = [ "html" "htm" ];
          language-servers = [ "vscode-html-language-server" ];
          formatter.command = "prettier";
          formatter.args = [ "--parser" "html" ];
        }
        {
          name = "css";
          scope = "source.css";
          file-types = [ "css" "scss" "sass" ];
          language-servers = [ "vscode-css-language-server" ];
          formatter.command = "prettier";
          formatter.args = [ "--parser" "css" ];
        }
        {
          name = "json";
          scope = "source.json";
          file-types = [ "json" "jsonc" ];
          language-servers = [ "vscode-json-language-server" ];
          formatter.command = "prettier";
          formatter.args = [ "--parser" "json" ];
        }
        {
          name = "lua";
          scope = "source.lua";
          file-types = [ "lua" ];
          language-servers = [ "lua-language-server" ];
          formatter.command = "stylua";
          formatter.args = [ "-" ];
        }
        {
          name = "c";
          scope = "source.c";
          file-types = [ "c" "h" ];
          language-servers = [ "clangd" ];
        }
        {
          name = "cpp";
          scope = "source.cpp";
          file-types = [ "cpp" "cxx" "cc" "hpp" "hxx" "hh" ];
          language-servers = [ "clangd" ];
        }
        {
          name = "zig";
          scope = "source.zig";
          file-types = [ "zig" ];
          language-servers = [ "zls" ];
        }
        {
          name = "markdown";
          scope = "source.md";
          file-types = [ "md" "markdown" ];
          language-servers = [ "marksman" ];
        }
        {
          name = "yaml";
          scope = "source.yaml";
          file-types = [ "yml" "yaml" ];
          language-servers = [ "yaml-language-server" ];
        }
        {
          name = "toml";
          scope = "source.toml";
          file-types = [ "toml" ];
          language-servers = [ "taplo" ];
        }
        {
          name = "bash";
          scope = "source.bash";
          file-types = [ "sh" "bash" "zsh" ];
          language-servers = [ "bash-language-server" ];
          formatter.command = "shfmt";
        }
        {
          name = "dockerfile";
          scope = "source.dockerfile";
          file-types = [ "dockerfile" "Dockerfile" ];
          language-servers = [ "dockerfile-language-server" ];
        }
        {
          name = "terraform";
          scope = "source.hcl";
          file-types = [ "tf" "hcl" ];
          language-servers = [ "terraform-ls" ];
        }
      ];
    };

    themes = {
      catppuccin_mocha = {
        "ui.background" = { bg = "#1e1e2e"; };
        "ui.text" = "#cdd6f4";
        "ui.text.focus" = "#f2cdcd";
        "ui.selection" = { bg = "#585b70"; };
        "ui.cursor" = { bg = "#f2cdcd"; fg = "#1e1e2e"; };
        "ui.cursor.match" = { bg = "#a6adc8"; fg = "#1e1e2e"; };
        "ui.linenr" = "#6c7086";
        "ui.linenr.selected" = "#fab387";
        "ui.statusline" = { bg = "#313244"; fg = "#cdd6f4"; };
        "ui.statusline.inactive" = { bg = "#181825"; fg = "#6c7086"; };
        "ui.popup" = { bg = "#313244"; };
        "ui.window" = "#6c7086";
        "ui.help" = { bg = "#313244"; fg = "#cdd6f4"; };
        "ui.menu" = { bg = "#313244"; fg = "#cdd6f4"; };
        "ui.menu.selected" = { bg = "#585b70"; };

        "diagnostic.error" = "#f38ba8";
        "diagnostic.warning" = "#fab387";
        "diagnostic.info" = "#89b4fa";
        "diagnostic.hint" = "#a6e3a1";

        "comment" = "#6c7086";
        "keyword" = "#cba6f7";
        "keyword.control" = "#cba6f7";
        "keyword.directive" = "#f9e2af";
        "operator" = "#89dceb";
        "punctuation" = "#bac2de";
        "string" = "#a6e3a1";
        "string.regexp" = "#f5c2e7";
        "constant" = "#fab387";
        "constant.numeric" = "#fab387";
        "constant.builtin" = "#fab387";
        "type" = "#f9e2af";
        "type.builtin" = "#f9e2af";
        "function" = "#89b4fa";
        "function.macro" = "#f38ba8";
        "variable" = "#cdd6f4";
        "variable.builtin" = "#f38ba8";
        "variable.parameter" = "#eba0ac";
        "tag" = "#f2cdcd";
        "attribute" = "#89b4fa";
        "namespace" = "#f2cdcd";

        "markup.heading" = "#89b4fa";
        "markup.bold" = { fg = "#f38ba8"; modifiers = ["bold"]; };
        "markup.italic" = { fg = "#f38ba8"; modifiers = ["italic"]; };
        "markup.strikethrough" = { modifiers = ["crossed_out"]; };
        "markup.link.url" = "#89b4fa";
        "markup.link.text" = "#74c7ec";
        "markup.quote" = "#a6e3a1";
        "markup.raw" = "#fab387";

        "diff.plus" = "#a6e3a1";
        "diff.minus" = "#f38ba8";
        "diff.delta" = "#fab387";
      };
    };
  };
}
