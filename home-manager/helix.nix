{ pkgs, ... }:

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
      };

      keys.normal = {
        space.space = "file_picker";
        space.w = ":w";
        space.q = ":q";
        esc = [ "collapse_selection" "keep_primary_selection" ];
      };
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