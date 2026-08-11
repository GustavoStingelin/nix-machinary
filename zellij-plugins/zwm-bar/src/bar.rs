//! The status line's rendering.
//!
//! Kept free of every Zellij host call and of the plugin API's types, so it can
//! be unit tested with the host toolchain — the crate itself builds for
//! wasm32-wasip1, where a test binary could not run. `main.rs` maps Zellij's
//! events onto the small inputs below.

/// Catppuccin Mocha as truecolor components, the palette of the zjstatus
/// configuration this bar replaces (see home-manager/zellij.nix).
const BASE: &str = "30;30;46"; // #1E1E2E — the bar's own background
const TEXT: &str = "205;214;244"; // #CDD6F4 — inactive tab text
const BLUE: &str = "137;180;250"; // #89B4FA — normal mode, active tab
const GREEN: &str = "166;227;161"; // #A6E3A1 — tmux mode, active+sync tab
const YELLOW: &str = "249;226;175"; // #F9E2AF — locked mode, active+fullscreen tab
const MAUVE: &str = "203;166;247"; // #CBA6F7 — the session block
const SURFACE0: &str = "49;50;68"; // #313244 — inactive tab
const SURFACE1: &str = "69;71;90"; // #45475A — inactive fullscreen tab
const OVERLAY0: &str = "108;112;134"; // #6C7086 — inactive sync tab

/// The Powerline separator (U+E0B8) closing every coloured block, carried over
/// verbatim from the replaced configuration. It needs a patched font, which this
/// setup already relies on elsewhere.
const SEP: char = '\u{e0b8}';

/// Erase-to-end-of-line. Emitted after the content with the bar's background
/// selected, which fills the rest of the row without padding it by hand — the
/// same trick Zellij's own compact-bar uses.
const CLEAR_TO_END: &str = "\u{1b}[0K";

/// Which mode colour to draw. The replaced configuration coloured only these
/// three and left every other mode unstyled; anything else now takes the normal
/// colour instead, because an unstyled block in the middle of the bar reads as a
/// glitch rather than as information.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ModeColour {
    Normal,
    Tmux,
    Locked,
}

/// One tab, reduced to what the styling actually depends on. `index` is the
/// 1-based display position, as the bar shows it.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Tab {
    pub index: usize,
    pub name: String,
    pub active: bool,
    pub fullscreen: bool,
    pub sync: bool,
}

/// Render the whole line: the input mode, the session name, then the tabs, with
/// the bar's background filling whatever is left of `cols`.
///
/// `mode` and `session` are optional because a freshly loaded bar has received
/// no ModeUpdate yet; it draws an empty line rather than guessing.
pub fn render(mode: Option<(ModeColour, &str)>, session: Option<&str>, tabs: &[Tab], cols: usize) -> String {
    let mut line = String::new();

    if let Some((colour, label)) = mode {
        let colour = match colour {
            ModeColour::Normal => BLUE,
            ModeColour::Tmux => GREEN,
            ModeColour::Locked => YELLOW,
        };
        line.push_str(&style(BASE, colour, true, false));
        line.push_str(&format!(" {label} "));
        line.push_str(&style(colour, BASE, false, false));
        line.push(SEP);
    }

    if let Some(session) = session {
        line.push_str(&style(BASE, MAUVE, true, false));
        line.push_str(&format!(" {session} "));
        line.push_str(&style(MAUVE, BASE, false, false));
        line.push(SEP);
    }

    for tab in tabs {
        line.push_str(&tab_block(tab));
    }

    fit(line, cols)
}

/// One tab's block. The active tab is bold italic on a bright background with no
/// leading gap; the others carry the extra space the old format string had.
/// Fullscreen beats sync when a tab is both, matching the precedence zjstatus
/// applied to the same format keys.
fn tab_block(tab: &Tab) -> String {
    let Tab { index, name, .. } = tab;
    if tab.active {
        let colour = if tab.fullscreen {
            YELLOW
        } else if tab.sync {
            GREEN
        } else {
            BLUE
        };
        format!(
            "{}{SEP}{index} {name} {}{SEP}",
            style(BASE, colour, true, true),
            style(colour, BASE, false, false),
        )
    } else {
        let colour = if tab.fullscreen {
            SURFACE1
        } else if tab.sync {
            OVERLAY0
        } else {
            SURFACE0
        };
        format!(
            "{}{SEP} {} {index} {name} {}{SEP}",
            style(BASE, colour, false, false),
            style(TEXT, colour, false, false),
            style(colour, BASE, false, false),
        )
    }
}

/// Select a foreground/background pair, resetting first so attributes from the
/// previous block never leak into this one.
fn style(fg: &str, bg: &str, bold: bool, italic: bool) -> String {
    let mut out = format!("\u{1b}[0m\u{1b}[38;2;{fg}m\u{1b}[48;2;{bg}m");
    if bold {
        out.push_str("\u{1b}[1m");
    }
    if italic {
        out.push_str("\u{1b}[3m");
    }
    out
}

/// Fill the row out to `cols`, or clip the content there.
///
/// Width counts visible characters: escape sequences are copied through without
/// consuming columns. Characters are counted rather than measured, so a tab name
/// made of double-width glyphs can still overflow — a wide CJK tab name would
/// need a unicode-width dependency to place exactly, which is not worth carrying
/// into the wasm for a bar whose overflow behaviour is to be cut off anyway.
fn fit(line: String, cols: usize) -> String {
    let mut out = String::with_capacity(line.len() + 16);
    let mut width = 0;
    let mut chars = line.chars();

    while let Some(character) = chars.next() {
        if character == '\u{1b}' {
            out.push(character);
            // A CSI sequence ends at its first letter; copy it verbatim.
            for next in chars.by_ref() {
                out.push(next);
                if next.is_ascii_alphabetic() {
                    break;
                }
            }
            continue;
        }
        if width == cols {
            // No room left: drop the remaining content, styling included.
            break;
        }
        out.push(character);
        width += 1;
    }

    if width < cols {
        // Let the terminal paint the remainder in the bar's own background.
        out.push_str(&style(TEXT, BASE, false, false));
        out.push_str(CLEAR_TO_END);
    }
    out.push_str("\u{1b}[0m");
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Visible width of a rendered line: everything outside the escape
    /// sequences. Mirrors what a terminal would advance the cursor by.
    fn visible(line: &str) -> String {
        let mut out = String::new();
        let mut chars = line.chars();
        while let Some(character) = chars.next() {
            if character == '\u{1b}' {
                for next in chars.by_ref() {
                    if next.is_ascii_alphabetic() {
                        break;
                    }
                }
                continue;
            }
            out.push(character);
        }
        out
    }

    fn tab(index: usize, name: &str, active: bool) -> Tab {
        Tab { index, name: name.to_owned(), active, fullscreen: false, sync: false }
    }

    #[test]
    fn renders_mode_session_and_tabs_in_that_order() {
        let tabs = [tab(1, "btcwallet", true), tab(2, "lnd", false)];
        let line = render(Some((ModeColour::Normal, "Normal")), Some("bitcoin"), &tabs, 80);
        let text = visible(&line);

        let mode = text.find("Normal").expect("mode label");
        let session = text.find("bitcoin").expect("session name");
        let first = text.find("btcwallet").expect("first tab");
        let second = text.find("lnd").expect("second tab");
        assert!(mode < session && session < first && first < second, "unexpected order in {text:?}");
    }

    #[test]
    fn shows_the_one_based_tab_index_beside_the_name() {
        let line = render(None, None, &[tab(2, "lnd", false)], 40);
        assert!(visible(&line).contains("2 lnd"), "{:?}", visible(&line));
    }

    #[test]
    fn draws_the_active_tab_bold_italic_and_the_others_plain() {
        let active = render(None, None, &[tab(1, "one", true)], 40);
        assert!(active.contains("\u{1b}[1m\u{1b}[3m"), "active tab should be bold italic");

        let inactive = render(None, None, &[tab(1, "one", false)], 40);
        assert!(!inactive.contains("\u{1b}[3m"), "inactive tab should not be italic");
    }

    #[test]
    fn fullscreen_wins_over_sync_on_the_same_tab() {
        let both = Tab { fullscreen: true, sync: true, ..tab(1, "one", true) };
        let line = render(None, None, &[both], 40);
        assert!(line.contains(YELLOW), "fullscreen colour should win");
        assert!(!line.contains(GREEN), "sync colour should not appear");
    }

    #[test]
    fn a_line_shorter_than_the_row_is_left_for_the_terminal_to_fill() {
        let line = render(Some((ModeColour::Normal, "Normal")), Some("bitcoin"), &[tab(1, "one", true)], 200);
        assert!(line.ends_with(&format!("{CLEAR_TO_END}\u{1b}[0m")), "should clear to end of row");
        assert!(visible(&line).chars().count() < 200);
    }

    #[test]
    fn a_line_longer_than_the_row_is_clipped_to_it() {
        let tabs: Vec<Tab> = (1..=20).map(|i| tab(i, "a-fairly-long-tab-name", false)).collect();
        let line = render(Some((ModeColour::Normal, "Normal")), Some("bitcoin"), &tabs, 60);

        assert_eq!(visible(&line).chars().count(), 60);
        assert!(!line.contains(CLEAR_TO_END), "a clipped line has nothing left to fill");
        assert!(line.ends_with("\u{1b}[0m"), "styling must not leak past the bar");
    }

    #[test]
    fn a_bar_that_has_seen_no_mode_update_yet_draws_an_empty_row() {
        let line = render(None, None, &[], 40);
        assert_eq!(visible(&line), "");
        assert!(line.contains(CLEAR_TO_END));
    }

    #[test]
    fn each_mode_keeps_its_own_colour() {
        for (colour, expected) in [
            (ModeColour::Normal, BLUE),
            (ModeColour::Tmux, GREEN),
            (ModeColour::Locked, YELLOW),
        ] {
            let line = render(Some((colour, "Mode")), None, &[], 40);
            assert!(line.contains(expected), "{colour:?} should use {expected}");
        }
    }
}
