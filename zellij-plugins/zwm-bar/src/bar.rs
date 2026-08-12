//! The status line's rendering.
//!
//! Kept free of every Zellij host call and of the plugin API's types, so it can
//! be unit tested with the host toolchain — the crate itself builds for
//! wasm32-wasip1, where a test binary could not run. `main.rs` maps Zellij's
//! events onto the small inputs below.

use zwm_tabmark::{Mark, SPINNER};

/// Catppuccin Mocha as truecolor components, the palette of the zjstatus
/// configuration this bar replaces (see home-manager/zellij.nix).
const BASE: &str = "30;30;46"; // #1E1E2E — the bar's own background
const TEXT: &str = "205;214;244"; // #CDD6F4 — inactive tab text
const BLUE: &str = "137;180;250"; // #89B4FA — normal mode, active tab
const GREEN: &str = "166;227;161"; // #A6E3A1 — tmux mode, active+sync tab
const YELLOW: &str = "249;226;175"; // #F9E2AF — locked mode, active+fullscreen tab
const MAUVE: &str = "203;166;247"; // #CBA6F7 — the session block
const MAROON: &str = "235;160;172"; // #EBA0AC — an agent waiting for the user
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
/// 1-based display position, as the bar shows it, and `name` is the tab's title
/// with the state marker already split off into `mark`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Tab {
    pub index: usize,
    pub name: String,
    pub active: bool,
    pub fullscreen: bool,
    pub sync: bool,
    /// The attention state of the agents in this tab, from its name's marker.
    pub mark: Mark,
}

/// Render the whole line: the input mode, the session name, then the tabs, with
/// the bar's background filling whatever is left of `cols`.
///
/// `mode` and `session` are optional because a freshly loaded bar has received
/// no ModeUpdate yet; it draws an empty line rather than guessing. `frame`
/// selects the spinner frame for every working tab, so they all turn together.
pub fn render(
    mode: Option<(ModeColour, &str)>,
    session: Option<&str>,
    tabs: &[Tab],
    cols: usize,
    frame: usize,
) -> String {
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
        line.push_str(&tab_block(tab, frame));
    }

    fit(line, cols)
}

/// One tab's block. The active tab is bold italic on a bright background with no
/// leading gap; the others carry the extra space the old format string had.
/// Fullscreen beats sync when a tab is both, matching the precedence zjstatus
/// applied to the same format keys.
fn tab_block(tab: &Tab, frame: usize) -> String {
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
            "{}{SEP}{index} {}{name} {}{SEP}",
            style(BASE, colour, true, true),
            mark_slot(tab, frame, colour),
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
            "{}{SEP} {} {index} {}{name} {}{SEP}",
            style(BASE, colour, false, false),
            style(TEXT, colour, false, false),
            mark_slot(tab, frame, colour),
            style(colour, BASE, false, false),
        )
    }
}

/// The two columns the agent-state marker occupies, over a tab block whose
/// background is `bg`: the state's glyph and a space. Empty for a tab no agent
/// has run in, which reserves no slot at all, and two blank columns once the
/// marker has been seen — that blank is what keeps a tab's width steady as its
/// state changes (see the zwm-tabmark crate).
///
/// On the active tab the glyph keeps the block's own foreground instead of the
/// state colour: that block's background is one of the bright accents, and a blue
/// spinner on a blue background is invisible. Its shape still says which state it
/// is — and the notification states never appear there anyway, since focusing a
/// tab dismisses them.
fn mark_slot(tab: &Tab, frame: usize, bg: &str) -> String {
    let glyph = match tab.mark {
        Mark::Unmarked => return String::new(),
        Mark::Cleared => return Mark::Cleared.prefix().to_owned(),
        // Every working tab in the bar turns on the same frame.
        Mark::Working => SPINNER[frame % SPINNER.len()],
        mark => mark.glyph().unwrap_or(' '),
    };
    if tab.active {
        return format!("{glyph} ");
    }
    format!(
        "{}{glyph} {}",
        style(mark_colour(tab.mark), bg, false, false),
        style(TEXT, bg, false, false),
    )
}

/// The state colours, matching the dashboard's (styles.go in zwm/internal/tui) so
/// a state reads the same in the bar and in `zwm tui`.
fn mark_colour(mark: Mark) -> &'static str {
    match mark {
        Mark::Working => BLUE,
        Mark::Waiting => MAROON,
        Mark::Done => GREEN,
        Mark::Unmarked | Mark::Cleared => TEXT,
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
        Tab { index, name: name.to_owned(), active, fullscreen: false, sync: false, mark: Mark::Unmarked }
    }

    #[test]
    fn renders_mode_session_and_tabs_in_that_order() {
        let tabs = [tab(1, "btcwallet", true), tab(2, "lnd", false)];
        let line = render(Some((ModeColour::Normal, "Normal")), Some("bitcoin"), &tabs, 80, 0);
        let text = visible(&line);

        let mode = text.find("Normal").expect("mode label");
        let session = text.find("bitcoin").expect("session name");
        let first = text.find("btcwallet").expect("first tab");
        let second = text.find("lnd").expect("second tab");
        assert!(mode < session && session < first && first < second, "unexpected order in {text:?}");
    }

    #[test]
    fn shows_the_one_based_tab_index_beside_the_name() {
        let line = render(None, None, &[tab(2, "lnd", false)], 40, 0);
        assert!(visible(&line).contains("2 lnd"), "{:?}", visible(&line));
    }

    #[test]
    fn draws_the_active_tab_bold_italic_and_the_others_plain() {
        let active = render(None, None, &[tab(1, "one", true)], 40, 0);
        assert!(active.contains("\u{1b}[1m\u{1b}[3m"), "active tab should be bold italic");

        let inactive = render(None, None, &[tab(1, "one", false)], 40, 0);
        assert!(!inactive.contains("\u{1b}[3m"), "inactive tab should not be italic");
    }

    #[test]
    fn fullscreen_wins_over_sync_on_the_same_tab() {
        let both = Tab { fullscreen: true, sync: true, ..tab(1, "one", true) };
        let line = render(None, None, &[both], 40, 0);
        assert!(line.contains(YELLOW), "fullscreen colour should win");
        assert!(!line.contains(GREEN), "sync colour should not appear");
    }

    #[test]
    fn a_line_shorter_than_the_row_is_left_for_the_terminal_to_fill() {
        let line = render(Some((ModeColour::Normal, "Normal")), Some("bitcoin"), &[tab(1, "one", true)], 200, 0);
        assert!(line.ends_with(&format!("{CLEAR_TO_END}\u{1b}[0m")), "should clear to end of row");
        assert!(visible(&line).chars().count() < 200);
    }

    #[test]
    fn a_line_longer_than_the_row_is_clipped_to_it() {
        let tabs: Vec<Tab> = (1..=20).map(|i| tab(i, "a-fairly-long-tab-name", false)).collect();
        let line = render(Some((ModeColour::Normal, "Normal")), Some("bitcoin"), &tabs, 60, 0);

        assert_eq!(visible(&line).chars().count(), 60);
        assert!(!line.contains(CLEAR_TO_END), "a clipped line has nothing left to fill");
        assert!(line.ends_with("\u{1b}[0m"), "styling must not leak past the bar");
    }

    #[test]
    fn a_bar_that_has_seen_no_mode_update_yet_draws_an_empty_row() {
        let line = render(None, None, &[], 40, 0);
        assert_eq!(visible(&line), "");
        assert!(line.contains(CLEAR_TO_END));
    }

    fn marked(name: &str, mark: Mark, active: bool) -> Tab {
        Tab { mark, ..tab(1, name, active) }
    }

    #[test]
    fn an_agent_state_is_drawn_as_its_glyph_before_the_tab_name() {
        for (mark, glyph) in [(Mark::Working, '◐'), (Mark::Waiting, '●'), (Mark::Done, '✓')] {
            let line = render(None, None, &[marked("btcwallet", mark, false)], 40, 0);
            assert!(
                visible(&line).contains(&format!("1 {glyph} btcwallet")),
                "{mark:?} should draw {glyph}: {:?}",
                visible(&line)
            );
        }
    }

    #[test]
    fn each_state_keeps_the_colour_the_dashboard_gives_it() {
        for (mark, expected) in [(Mark::Working, BLUE), (Mark::Waiting, MAROON), (Mark::Done, GREEN)] {
            let line = render(None, None, &[marked("btcwallet", mark, false)], 40, 0);
            assert!(line.contains(expected), "{mark:?} should use {expected}");
        }
    }

    #[test]
    fn a_working_tab_turns_through_the_spinner_frames() {
        let frames: Vec<String> = (0..5)
            .map(|frame| {
                let line = render(None, None, &[marked("btcwallet", Mark::Working, false)], 40, frame);
                visible(&line).chars().find(|c| SPINNER.contains(c)).expect("a spinner glyph").into()
            })
            .collect();

        assert_eq!(frames, ["◐", "◓", "◑", "◒", "◐"], "frames should advance and wrap");
    }

    #[test]
    fn the_spinner_on_the_active_tab_keeps_the_block_foreground() {
        // Its background is the bright accent colour, so the state colour would
        // be all but invisible there.
        let line = render(None, None, &[marked("btcwallet", Mark::Working, true)], 40, 0);
        assert!(visible(&line).contains("1 ◐ btcwallet"), "{:?}", visible(&line));
        let blue_on_blue = format!("\u{1b}[38;2;{BLUE}m\u{1b}[48;2;{BLUE}m");
        assert!(!line.contains(&blue_on_blue), "the glyph must not be drawn blue on a blue block");
    }

    #[test]
    fn a_state_change_never_changes_a_tabs_width() {
        // What the blank cleared marker is for: the tabs to the right of a tab
        // whose agent just finished must not shift.
        let widths: Vec<usize> = [Mark::Cleared, Mark::Working, Mark::Waiting, Mark::Done]
            .into_iter()
            .map(|mark| {
                let line = render(None, None, &[marked("btcwallet", mark, false)], 60, 0);
                visible(&line).trim_end().chars().count()
            })
            .collect();

        assert!(widths.windows(2).all(|pair| pair[0] == pair[1]), "widths differed: {widths:?}");
    }

    #[test]
    fn a_tab_no_agent_has_run_in_reserves_no_room_for_a_marker() {
        let plain = render(None, None, &[marked("btcwallet", Mark::Unmarked, false)], 60, 0);
        let cleared = render(None, None, &[marked("btcwallet", Mark::Cleared, false)], 60, 0);

        assert!(visible(&plain).contains("1 btcwallet"), "{:?}", visible(&plain));
        assert_eq!(
            visible(&cleared).trim_end().chars().count(),
            visible(&plain).trim_end().chars().count() + 2,
            "a marked tab keeps its two columns; an unmarked one never takes them"
        );
    }

    #[test]
    fn each_mode_keeps_its_own_colour() {
        for (colour, expected) in [
            (ModeColour::Normal, BLUE),
            (ModeColour::Tmux, GREEN),
            (ModeColour::Locked, YELLOW),
        ] {
            let line = render(Some((colour, "Mode")), None, &[], 40, 0);
            assert!(line.contains(expected), "{colour:?} should use {expected}");
        }
    }
}
