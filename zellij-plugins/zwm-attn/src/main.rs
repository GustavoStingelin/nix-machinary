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
//! glyph so it stands out in the (zjstatus) tab bar. The glyph is stripped
//! automatically when the user focuses the tab — the glyph in the tab name is
//! the entire state, so no bookkeeping is needed.
//!
//! The same pipe also serves `event=focus`, which focuses the pane instead of
//! marking its tab. That exists because the Zellij 0.43.1 CLI can focus a tab
//! by name but not a pane by id, and the `zwm tui` dashboard wants Enter on an
//! agent to land on the agent's own pane.

use std::collections::BTreeMap;

use zellij_tile::prelude::*;

/// Leading marker added to a tab name. zjstatus renders the raw tab name, so
/// this shows verbatim in the bar. The trailing space keeps it legible.
const GLYPH: &str = "● ";

/// Pipe name the completion hooks address (`zellij pipe --name`).
const PIPE_NAME: &str = "zwm-attn";

/// `event=` value asking for a pane to be focused rather than for its tab to be
/// marked. Every other value is an agent attention state (working/waiting/done).
/// Must stay in sync with focusEvent in zwm/internal/zellij/inventory.go.
const FOCUS_EVENT: &str = "focus";

#[derive(Default)]
struct State {
    tabs: Vec<TabInfo>,
    panes: PaneManifest,
    /// terminal pane id (matches `$ZELLIJ_PANE_ID`) -> tab display index (0-based)
    pane_to_tab: BTreeMap<u32, usize>,
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
    }

    fn update(&mut self, event: Event) -> bool {
        match event {
            Event::TabUpdate(tabs) => {
                self.tabs = tabs;
                self.rebuild_pane_map();
                // Focusing a marked tab is the "I've seen it" signal.
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
        let Some(tab) = self.tabs.get(index) else {
            return false;
        };
        // Nothing to signal if the user is already looking at the tab, and
        // never stack the glyph on repeated events.
        if tab.active || tab.name.starts_with(GLYPH) {
            return false;
        }
        rename_tab(display_position(index), format!("{GLYPH}{}", tab.name));
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

    /// Strip the glyph from the currently focused tab, if present.
    fn clear_focused(&self) {
        for (index, tab) in self.tabs.iter().enumerate() {
            if tab.active {
                if let Some(stripped) = tab.name.strip_prefix(GLYPH) {
                    rename_tab(display_position(index), stripped.to_string());
                }
            }
        }
    }
}

/// `rename_tab` addresses tabs by 1-based position; our indices are 0-based.
fn display_position(index: usize) -> u32 {
    index as u32 + 1
}
