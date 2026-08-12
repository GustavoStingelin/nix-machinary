//! zwm-attn — a headless Zellij plugin that surfaces a "waiting for you"
//! indicator on tabs.
//!
//! An agent-completion hook (Claude Code / opencode) running inside a pane
//! sends a CLI pipe carrying its own `$ZELLIJ_PANE_ID`:
//!
//! ```sh
//! zellij pipe --plugin zwm-attn --name zwm-attn --args "pane_id=$ZELLIJ_PANE_ID,event=finished"
//! ```
//!
//! This plugin maps that pane id to its tab and prefixes the tab name with a
//! marker carrying the agent's state, which zwm-bar reads back and draws as a
//! glyph — a turning spinner while the agent works, a dot when it is blocked on
//! the user, a check when it is done. The marker in the tab name is the entire
//! state, so this plugin keeps no bookkeeping of its own; the vocabulary and the
//! transitions live in the zwm-tabmark crate, which both plugins share.
//!
//! The same pipe also serves `event=focus`, which focuses the pane instead of
//! marking its tab. That exists because the Zellij 0.43.1 CLI can focus a tab
//! by name but not a pane by id, and the `zwm tui` dashboard wants Enter on an
//! agent to land on the agent's own pane.

use std::collections::BTreeMap;

use zellij_tile::prelude::*;
use zwm_tabmark as tabmark;

/// Pipe name the completion hooks address (`zellij pipe --name`).
const PIPE_NAME: &str = "zwm-attn";

/// `event=` value asking for a pane to be focused rather than for its tab to be
/// marked. Every other value is an agent attention state, decided by
/// `tabmark::Mark::from_signal`. Must stay in sync with focusEvent in
/// zwm/internal/zellij/pipe.go.
const FOCUS_EVENT: &str = "focus";

/// First Zellij release where `rename_tab` addresses the tab at a *display
/// position*, as its API documents.
///
/// Older hosts (0.43.x and down) resolve the argument against `screen.tabs`,
/// a map keyed by an internal tab index: closing a non-last tab shifts every
/// later tab's position down but leaves a permanent hole in the index keys, so
/// the two numbers drift apart and a rename silently lands on an unrelated tab,
/// overwriting its name. Upstream fixed it by switching to
/// `get_tab_by_position_mut`. A plugin cannot see the internal index, so there
/// is nothing to correct for — on an older host we simply don't rename. A tab
/// missing its glyph is a nuisance; a tab that loses its name is data loss.
const MIN_RENAME_VERSION: (u32, u32) = (0, 44);

#[derive(Default)]
struct State {
    tabs: Vec<TabInfo>,
    panes: PaneManifest,
    /// terminal pane id (matches `$ZELLIJ_PANE_ID`) -> tab display index (0-based)
    pane_to_tab: BTreeMap<u32, usize>,
    /// Whether this host renames the tab we actually mean (see MIN_RENAME_VERSION).
    can_rename_tabs: bool,
    /// Whether the first tab list has been swept of spinners left behind by a
    /// session that died while an agent was working (tabmark::on_plugin_load).
    swept: bool,
}

register_plugin!(State);

impl ZellijPlugin for State {
    fn load(&mut self, _configuration: BTreeMap<String, String>) {
        // Reading tab/pane state and renaming tabs both need application-state
        // permissions. Receiving a CLI pipe does not require ReadCliPipes (that
        // only governs blocking/output on the pipe, which we don't use).
        request_permission(&[
            PermissionType::ReadApplicationState,
            PermissionType::ChangeApplicationState,
        ]);
        subscribe(&[
            EventType::TabUpdate,
            EventType::PaneUpdate,
            EventType::PermissionRequestResult,
        ]);
        let version = get_zellij_version();
        self.can_rename_tabs = renames_by_position(&version);
        if !self.can_rename_tabs {
            eprintln!(
                "zwm-attn: Zellij {version} renames tabs by internal index, \
                 not by position — attention glyphs are disabled to avoid \
                 renaming the wrong tab. Upgrade to {}.{}+.",
                MIN_RENAME_VERSION.0, MIN_RENAME_VERSION.1
            );
        }
    }

    fn update(&mut self, event: Event) -> bool {
        match event {
            Event::TabUpdate(tabs) => {
                self.tabs = tabs;
                self.rebuild_pane_map();
                // A resurrected session brings its tab names back verbatim, so
                // retire any spinner before it turns for an agent that is gone.
                // Only the first list needs it: from here on this plugin is the
                // only thing writing markers.
                if !self.swept {
                    self.swept = true;
                    self.sweep();
                }
                // Focusing a marked tab is the "I've seen it" signal. The sweep
                // above touches working markers only, so the two never race for
                // the same tab.
                self.clear_focused();
            }
            Event::PaneUpdate(panes) => {
                self.panes = panes;
                self.rebuild_pane_map();
            }
            _ => {}
        }
        // Headless plugin: never requests a render.
        false
    }

    fn pipe(&mut self, message: PipeMessage) -> bool {
        if message.name != PIPE_NAME {
            return false;
        }
        let Some(pane_id) = message
            .args
            .get("pane_id")
            .and_then(|value| value.parse::<u32>().ok())
        else {
            return false;
        };
        let Some(&index) = self.pane_to_tab.get(&pane_id) else {
            return false;
        };
        // Focus requests are done here: focusing a pane switches to its tab and
        // layer too, so the caller needs nothing else. Unknown ids never reach
        // this point, so a stale record can't steal focus.
        if message.args.get("event").map(String::as_str) == Some(FOCUS_EVENT) {
            focus_terminal_pane(pane_id, false, false);
            return false;
        }
        if !self.can_rename_tabs {
            return false;
        }
        let Some(tab) = self.tabs.get(index) else {
            return false;
        };
        // Which marker this signal calls for — including none at all, when the
        // tab already says what the signal would say (agents signal on every
        // turn, and every rename wakes every plugin in the session).
        let signal = message.args.get("event").map(String::as_str).unwrap_or_default();
        if let Some(name) = tabmark::on_signal(&tab.name, signal, tab.active) {
            rename_tab(display_position(index), name);
        }
        false
    }

    fn render(&mut self, _rows: usize, _cols: usize) {}
}

impl State {
    /// Rebuild the pane-id -> tab-index map from the latest tab and pane state.
    /// `PaneManifest.panes` is keyed by tab position; we record the terminal
    /// panes (skipping plugin/suppressed panes) against their tab's display
    /// index.
    fn rebuild_pane_map(&mut self) {
        self.pane_to_tab.clear();
        for (index, tab) in self.tabs.iter().enumerate() {
            let Some(panes) = self.panes.panes.get(&tab.position) else {
                continue;
            };
            for pane in panes {
                if !pane.is_plugin && !pane.is_suppressed {
                    self.pane_to_tab.insert(pane.id, index);
                }
            }
        }
    }

    /// Dismiss the notification on the currently focused tab, if it carries one.
    /// A working marker survives: it is the agent's status, not something the
    /// user needs to be told twice.
    fn clear_focused(&self) {
        self.retitle(|tab| if tab.active { tabmark::on_focus(&tab.name) } else { None });
    }

    /// Retire spinners inherited from a previous life of this session.
    fn sweep(&self) {
        self.retitle(|tab| tabmark::on_plugin_load(&tab.name));
    }

    /// Rename every tab for which `next` returns a new name.
    fn retitle(&self, next: impl Fn(&TabInfo) -> Option<String>) {
        if !self.can_rename_tabs {
            return;
        }
        for (index, tab) in self.tabs.iter().enumerate() {
            if let Some(name) = next(tab) {
                rename_tab(display_position(index), name);
            }
        }
    }
}

/// `rename_tab` addresses tabs by 1-based position; our indices are 0-based.
fn display_position(index: usize) -> u32 {
    index as u32 + 1
}

/// Whether the host's `rename_tab` targets a display position, i.e. whether it
/// is at least MIN_RENAME_VERSION. Anything unparseable counts as too old, so a
/// surprising version string costs a glyph rather than a tab name.
fn renames_by_position(version: &str) -> bool {
    let mut fields = version.trim().trim_start_matches('v').split('.');
    let leading_number = |field: Option<&str>| -> Option<u32> {
        let field = field?;
        let digits = field
            .split(|character: char| !character.is_ascii_digit())
            .next()?;
        digits.parse().ok()
    };
    match (leading_number(fields.next()), leading_number(fields.next())) {
        (Some(major), Some(minor)) => (major, minor) >= MIN_RENAME_VERSION,
        _ => false,
    }
}
