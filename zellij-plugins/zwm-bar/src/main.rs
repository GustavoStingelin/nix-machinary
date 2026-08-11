//! zwm-bar — the session's status line: input mode, session name, tabs.
//!
//! It replaces zjstatus, which had become the single largest CPU consumer in a
//! Zellij session here. zjstatus subscribes to `SessionUpdate`, and Zellij 0.44
//! emits that once a second carrying *every* live session's full tab and pane
//! manifest; with one bar instance per tab, nine instances each decoded that
//! payload inside the wasm interpreter and re-rendered, every second, whether or
//! not anything had changed. Sampling a nine-tab session with no user input
//! measured 94s of interpreter CPU per 150s of wall clock, all of it under
//! `apply_event_to_plugin`.
//!
//! This bar subscribes to two events instead. `ModeUpdate` carries the input
//! mode *and* the session name; `TabUpdate` carries the tabs. Between them that
//! is the entire bar, and neither fires while nothing changes — so an idle
//! session costs nothing at all. `update` also compares before reporting a
//! change, so a repeated event does not repaint.
//!
//! The colours, separators and spacing are ported from the zjstatus format
//! strings this replaces, so the bar looks the same. Rendering lives in `bar`.

use std::collections::BTreeMap;

use zellij_tile::prelude::*;
// Imported as a module, not by item: register_plugin! generates its own
// `render` export, which a bare `use ...::render` would collide with.
use zwm_bar::bar::{self, ModeColour, Tab};

#[derive(Default)]
struct State {
    /// None until the first ModeUpdate; the bar draws an empty row until then
    /// rather than inventing a mode.
    mode: Option<InputMode>,
    session: Option<String>,
    tabs: Vec<TabInfo>,
}

register_plugin!(State);

impl ZellijPlugin for State {
    fn load(&mut self, _configuration: BTreeMap<String, String>) {
        // The bar reads the mode and the tab list and changes nothing.
        request_permission(&[PermissionType::ReadApplicationState]);
        // Note what is *not* here: set_selectable(false). Zellij draws the
        // permission prompt in this pane and it has to be focused to answer it,
        // so an unselectable bar shows a "press y/n" prompt that cannot be
        // reached — the pane stays selectable until the request is answered,
        // which is where set_selectable moves to.
        subscribe(&[
            EventType::ModeUpdate,
            EventType::TabUpdate,
            EventType::PermissionRequestResult,
        ]);
    }

    fn update(&mut self, event: Event) -> bool {
        match event {
            Event::ModeUpdate(mode_info) => {
                // The session name only ever arrives here, so hold on to the
                // last one seen: a later ModeUpdate without it must not blank
                // the block.
                let session = mode_info.session_name.filter(|name| !name.is_empty());
                let changed =
                    self.mode != Some(mode_info.mode) || (session.is_some() && self.session != session);
                self.mode = Some(mode_info.mode);
                if session.is_some() {
                    self.session = session;
                }
                changed
            },
            Event::TabUpdate(tabs) => {
                let changed = self.tabs != tabs;
                self.tabs = tabs;
                changed
            },
            Event::PermissionRequestResult(_) => {
                // The prompt is done with this pane, so retire it from the focus
                // cycle: a status bar is not somewhere to land. Answered either
                // way — a denied bar draws an empty row, and still should not be
                // focusable.
                set_selectable(false);
                // Nothing could be drawn until now, so paint.
                true
            },
            _ => false,
        }
    }

    fn render(&mut self, _rows: usize, cols: usize) {
        let mode = self.mode.map(|mode| (mode_colour(mode), format!("{mode:?}")));
        let tabs: Vec<Tab> = self
            .tabs
            .iter()
            .map(|tab| Tab {
                index: tab.position + 1,
                name: tab.name.clone(),
                active: tab.active,
                fullscreen: tab.is_fullscreen_active,
                sync: tab.is_sync_panes_active,
            })
            .collect();
        print!(
            "{}",
            bar::render(
                mode.as_ref().map(|(colour, label)| (*colour, label.as_str())),
                self.session.as_deref(),
                &tabs,
                cols,
            )
        );
    }
}

/// Only normal, tmux and locked had colours in the configuration this replaces.
/// Every other mode (pane, tab, resize, search, …) takes the normal colour
/// rather than rendering unstyled, which looked like a fault.
fn mode_colour(mode: InputMode) -> ModeColour {
    match mode {
        InputMode::Locked => ModeColour::Locked,
        InputMode::Tmux => ModeColour::Tmux,
        _ => ModeColour::Normal,
    }
}
