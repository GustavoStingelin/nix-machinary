//! zwm-bar — the session's status line: input mode, session name, tabs, and the
//! attention state of the agents running in each tab.
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
//!
//! # The agent state glyphs, and what animating them costs
//!
//! zwm-attn keeps each tab's agent state in a marker at the front of the tab
//! name (see the zwm-tabmark crate), so it arrives here inside the `TabUpdate`
//! this bar already reads — no extra subscription, no extra permission. The bar
//! strips the marker and draws the state itself: a spinner turning while an agent
//! works, a maroon dot when it wants the user, a green check when it is done,
//! matching what `zwm tui` shows for the same agents.
//!
//! Turning the spinner needs a timer, and a timer is exactly the shape of the
//! problem that got zjstatus removed — so it is gated twice. It is armed only
//! while some tab is actually working, which is zero cost in an idle session, and
//! only in the bar the user can see: `Tab::visible` in zellij-server sends
//! `Event::Visible` to every plugin pane in a tab as it is switched to or away
//! from, and `apply_event_to_plugin` renders a plugin whenever its `update` asks
//! for it *without checking whether its tab is on screen*.
//!
//! Both gates matter, because a render here is not cheap: the zjstatus
//! measurement above works out to ~70ms of interpreter CPU per render. One
//! spinner ungated across a twelve-tab session would be ~96 renders a second.
//! Gated, it is eight, in the one pane that can show them.

use std::collections::BTreeMap;

use zellij_tile::prelude::*;
use zwm_tabmark::{self as tabmark, Mark};
// Imported as a module, not by item: register_plugin! generates its own
// `render` export, which a bare `use ...::render` would collide with.
use zwm_bar::bar::{self, ModeColour, Tab};

/// How long between spinner frames. Matches spinnerInterval in
/// zwm/internal/tui/model.go, so a working agent turns at the same rate in the
/// bar and in the dashboard.
const SPINNER_INTERVAL: f64 = 0.12;

#[derive(Default)]
struct State {
    /// None until the first ModeUpdate; the bar draws an empty row until then
    /// rather than inventing a mode.
    mode: Option<InputMode>,
    session: Option<String>,
    tabs: Vec<TabInfo>,
    /// Which spinner frame the working tabs are on.
    frame: usize,
    /// Whether this instance's tab is on screen; see the module docs.
    visible: bool,
    /// Whether a spinner timer is already pending, so events arriving between
    /// frames cannot stack up a second one.
    ticking: bool,
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
            EventType::Visible,
            EventType::Timer,
            EventType::PermissionRequestResult,
        ]);
        // `visible` starts false, and nothing here sets it: Zellij reports
        // visibility only when it *changes*, and there is no call to ask. Of the
        // two ways to be wrong, this is the cheap one. Guessing visible would have
        // every tab of a resurrected session animating — `apply_layout` announces
        // the tab clients are moved into and `add_client` announces nothing, so a
        // background tab restored from disk is never told it is not on screen, and
        // the guess would stand until the user happened to visit it. Guessing
        // invisible instead costs a spinner that stays still until the first tab
        // switch, which announces both tabs involved and settles it for good.
        // Reloading the plugin under a live session lands in that window too,
        // which is why `just reload-zellij-plugins` nudges the focus afterwards.
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
                // A rename is how an agent's state reaches this bar, so a tab may
                // have just started (or stopped) working.
                self.arm_spinner();
                changed
            },
            Event::Visible(visible) => {
                self.visible = visible;
                // Becoming visible starts the spinner; becoming invisible simply
                // stops it being re-armed once the pending frame fires. Zellij
                // repaints the pane from its grid on a tab switch, so there is
                // nothing to redraw here.
                self.arm_spinner();
                false
            },
            Event::Timer(_) => {
                self.ticking = false;
                if !self.spinning() {
                    return false;
                }
                self.frame = self.frame.wrapping_add(1);
                self.arm_spinner();
                true
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
            .map(|tab| {
                // The agent state travels in the tab name; the bar shows the
                // title and draws the state as a glyph of its own.
                let (mark, title) = tabmark::split(&tab.name);
                Tab {
                    index: tab.position + 1,
                    name: title.to_owned(),
                    active: tab.active,
                    fullscreen: tab.is_fullscreen_active,
                    sync: tab.is_sync_panes_active,
                    mark,
                }
            })
            .collect();
        print!(
            "{}",
            bar::render(
                mode.as_ref().map(|(colour, label)| (*colour, label.as_str())),
                self.session.as_deref(),
                &tabs,
                cols,
                self.frame,
            )
        );
    }
}

impl State {
    /// Whether this bar should be turning a spinner: some tab is working, and the
    /// user can see this instance.
    fn spinning(&self) -> bool {
        self.visible
            && self
                .tabs
                .iter()
                .any(|tab| tabmark::split(&tab.name).0 == Mark::Working)
    }

    /// Ask for the next spinner frame, unless one is already pending or there is
    /// nothing to animate.
    fn arm_spinner(&mut self) {
        if self.ticking || !self.spinning() {
            return;
        }
        self.ticking = true;
        set_timeout(SPINNER_INTERVAL);
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
