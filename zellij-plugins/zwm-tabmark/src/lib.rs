//! How an agent's attention state is carried in a Zellij tab name.
//!
//! zwm-attn writes a two-column marker at the front of the tab name and zwm-bar
//! reads it back, stripping it and drawing the state glyph itself. Both plugins
//! depend on this crate so the vocabulary and the transitions have one home; the
//! `zwm tui` dashboard parses the same markers in Go, in
//! zwm/internal/zellij/inventory.go, which must stay byte-for-byte in sync with
//! the constants below.
//!
//! # Why the tab name is the transport
//!
//! It is the one piece of per-tab state both plugins already have: zwm-attn can
//! write it with the permission it already holds, and every bar instance is
//! handed it in `TabUpdate` without subscribing to anything new. It also survives
//! a plugin reload and a session resurrection, and `zellij action
//! query-tab-names` exposes it outside the session, which is how the dashboard
//! sees it. Piping the state from zwm-attn to each bar instance instead would
//! need a further permission (MessageAndLaunchOtherPlugins), would be lost on
//! every reload, and would leave a bar that loads later — a new tab — with no
//! state for the tabs beside it.
//!
//! # Why every marker is two columns
//!
//! A marker that appears and disappears changes the tab's width, which shoves
//! every tab to its right two columns across; that reads as a glitch when it
//! happens ~120ms after the tab is already focused. So a seen marker retires to
//! [`Mark::Cleared`], a blank of the same width, rather than being removed. The
//! cost is a permanent two-column indent on any tab an agent has ever run in.

/// An agent is working: the bar animates this one.
pub const WORKING: &str = "◐ ";
/// The agent is blocked on the user.
pub const WAITING: &str = "● ";
/// The agent finished its turn.
pub const DONE: &str = "✓ ";
/// A marker the user has seen, kept as a blank to hold the tab's width.
pub const CLEARED: &str = "  ";

/// The state of the agents in one tab, as recorded in its name.
///
/// This is per *tab*, not per pane: it is the state of the last agent in the tab
/// to say anything. The dashboard, which reads the per-pane records in
/// ~/.local/state/zwm/agents, is where several agents in one tab are told apart.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Mark {
    /// No marker at all — no agent has ever signalled from this tab.
    Unmarked,
    Cleared,
    Working,
    Waiting,
    Done,
}

impl Mark {
    /// The tab-name prefix for this mark. Empty only for [`Mark::Unmarked`].
    pub fn prefix(self) -> &'static str {
        match self {
            Mark::Unmarked => "",
            Mark::Cleared => CLEARED,
            Mark::Working => WORKING,
            Mark::Waiting => WAITING,
            Mark::Done => DONE,
        }
    }

    /// The glyph the bar draws for this mark, or None when the slot is blank.
    /// The working glyph is only the first spinner frame; see [`SPINNER`].
    pub fn glyph(self) -> Option<char> {
        // Taken from the marker itself rather than written out again, so the two
        // cannot drift apart.
        match self {
            Mark::Unmarked | Mark::Cleared => None,
            _ => self.prefix().chars().next(),
        }
    }

    /// Whether this mark is a notification — something the user has not seen yet,
    /// which focusing the tab therefore dismisses. `Working` is not: it is a
    /// running agent's status, and stays put while the user watches it.
    pub fn is_notification(self) -> bool {
        matches!(self, Mark::Waiting | Mark::Done)
    }

    /// The mark a pipe's `event=` value asks for. Unknown values yield None so a
    /// garbled event leaves the tab name alone rather than clearing it.
    ///
    /// The accepted values are the states `zwm attn` records (working/waiting/
    /// done) plus "closed", the pseudo-state an editor's exit hook sends: an
    /// agent that has gone away has no state to show, so its marker is retired.
    /// Must stay in sync with agentstate.State and closedSignal in
    /// zwm/internal/command/attn.go.
    pub fn from_signal(signal: &str) -> Option<Mark> {
        match signal {
            "working" => Some(Mark::Working),
            "waiting" => Some(Mark::Waiting),
            "done" => Some(Mark::Done),
            "closed" => Some(Mark::Cleared),
            _ => None,
        }
    }
}

/// The spinner frames for a working tab, in order — the same four the dashboard
/// turns (spinnerFrames in zwm/internal/tui/view.go), so "something is running"
/// reads identically in the bar and in the dashboard.
pub const SPINNER: [char; 4] = ['◐', '◓', '◑', '◒'];

/// Split a tab name into its marker and the bare title.
///
/// A tab name that genuinely begins with two spaces is indistinguishable from a
/// cleared marker. That is not new — it is how the marker has always worked — and
/// costs such a tab two columns of indent in the bar.
pub fn split(name: &str) -> (Mark, &str) {
    for mark in [Mark::Working, Mark::Waiting, Mark::Done, Mark::Cleared] {
        if let Some(title) = name.strip_prefix(mark.prefix()) {
            return (mark, title);
        }
    }
    (Mark::Unmarked, name)
}

/// The name a tab should take when `signal` arrives for an agent inside it, or
/// None to leave the name alone. `active` is whether the user is looking at the
/// tab right now.
///
/// Working is recorded whether or not the tab is active, because the signal
/// arrives when the user submits a prompt — almost always in the tab they are
/// looking at. Skipping it there, as the notification marks are skipped, would
/// mean the spinner never appeared at all.
///
/// The notification marks are for tabs the user is *not* looking at; on the
/// active tab they instead retire whatever marker is there, which is what stops
/// a spinner the moment the agent it belongs to finishes.
pub fn on_signal(name: &str, signal: &str, active: bool) -> Option<String> {
    let want = match Mark::from_signal(signal)? {
        mark if mark.is_notification() && active => Mark::Cleared,
        mark => mark,
    };
    retitle(name, want)
}

/// The name a tab should take once the user focuses it: a notification they have
/// now seen retires to the blank marker. None leaves the name alone, which is the
/// answer for an unmarked tab and for a working one.
pub fn on_focus(name: &str) -> Option<String> {
    let (mark, _) = split(name);
    if !mark.is_notification() {
        return None;
    }
    retitle(name, Mark::Cleared)
}

/// The name a tab should take when a fresh zwm-attn instance takes over the
/// session: a working marker is retired, every other mark is left alone.
///
/// A resurrected session's tab names come back from disk exactly as they were
/// saved, so a session killed while an agent was working would otherwise come
/// back with a tab spinning for an agent that no longer exists — and nothing
/// clears a working marker but the agent itself. Notifications are kept: an
/// unanswered "waiting for you" from before the session died is still worth
/// showing. The cost is that reloading the plugin under a live agent (see `just
/// reload-zellij-plugins`) drops its spinner until its next signal.
pub fn on_plugin_load(name: &str) -> Option<String> {
    let (mark, _) = split(name);
    if mark != Mark::Working {
        return None;
    }
    retitle(name, Mark::Cleared)
}

/// Re-prefix `name` with `want`, or None when it already carries that mark.
///
/// Returning None matters beyond saving a rename: every rename broadcasts a
/// `TabUpdate` to zwm-attn and to every bar instance in the session, so a
/// no-op rename would wake all of them.
///
/// Clearing an unmarked tab is also None: reserving the blank slot on a tab no
/// agent has ever run in would indent it forever for nothing.
fn retitle(name: &str, want: Mark) -> Option<String> {
    let (mark, title) = split(name);
    if mark == want || (want == Mark::Cleared && mark == Mark::Unmarked) {
        return None;
    }
    Some(format!("{}{title}", want.prefix()))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn every_marker_is_two_columns_so_a_state_change_never_reflows_the_bar() {
        for mark in [Mark::Cleared, Mark::Working, Mark::Waiting, Mark::Done] {
            assert_eq!(mark.prefix().chars().count(), 2, "{mark:?}");
        }
        assert_eq!(Mark::Unmarked.prefix(), "");
    }

    #[test]
    fn splits_a_marked_name_into_its_mark_and_bare_title() {
        for mark in [Mark::Cleared, Mark::Working, Mark::Waiting, Mark::Done] {
            let name = format!("{}btcwallet:pr-1314", mark.prefix());
            assert_eq!(split(&name), (mark, "btcwallet:pr-1314"), "{mark:?}");
        }
        assert_eq!(split("btcwallet"), (Mark::Unmarked, "btcwallet"));
    }

    #[test]
    fn a_working_signal_marks_the_tab_the_user_is_looking_at() {
        // The working signal fires when a prompt is submitted, so the tab is
        // nearly always active: skipping it here would hide every spinner.
        assert_eq!(on_signal("btcwallet", "working", true), Some("◐ btcwallet".into()));
        assert_eq!(on_signal("btcwallet", "working", false), Some("◐ btcwallet".into()));
    }

    #[test]
    fn a_notification_marks_only_a_tab_the_user_is_not_looking_at() {
        for (signal, prefix) in [("waiting", WAITING), ("done", DONE)] {
            assert_eq!(
                on_signal("btcwallet", signal, false),
                Some(format!("{prefix}btcwallet")),
                "{signal} on an inactive tab"
            );
            assert_eq!(
                on_signal("btcwallet", signal, true),
                None,
                "{signal} on an active, never-marked tab has nothing to say"
            );
        }
    }

    #[test]
    fn finishing_in_the_active_tab_retires_the_spinner_rather_than_leaving_it_turning() {
        for signal in ["waiting", "done", "closed"] {
            assert_eq!(
                on_signal("◐ btcwallet", signal, true),
                Some("  btcwallet".into()),
                "{signal} must stop the spinner it supersedes"
            );
        }
    }

    #[test]
    fn a_marker_replaces_the_one_before_it_instead_of_stacking() {
        assert_eq!(on_signal("◐ btcwallet", "done", false), Some("✓ btcwallet".into()));
        assert_eq!(on_signal("  btcwallet", "working", false), Some("◐ btcwallet".into()));
        assert_eq!(on_signal("✓ btcwallet", "waiting", false), Some("● btcwallet".into()));
    }

    #[test]
    fn repeating_a_signal_does_not_rename_the_tab() {
        // Agents signal on every turn, and each rename wakes every plugin in the
        // session with a TabUpdate.
        assert_eq!(on_signal("◐ btcwallet", "working", false), None);
        assert_eq!(on_signal("● btcwallet", "waiting", false), None);
        assert_eq!(on_signal("✓ btcwallet", "done", false), None);
    }

    #[test]
    fn an_unknown_signal_leaves_the_name_alone() {
        assert_eq!(on_signal("● btcwallet", "", false), None);
        assert_eq!(on_signal("● btcwallet", "finished", false), None);
    }

    #[test]
    fn a_closed_agent_takes_its_marker_with_it() {
        assert_eq!(on_signal("● btcwallet", "closed", false), Some("  btcwallet".into()));
        assert_eq!(on_signal("◐ btcwallet", "closed", false), Some("  btcwallet".into()));
    }

    #[test]
    fn clearing_never_indents_a_tab_no_agent_has_run_in() {
        assert_eq!(on_signal("btcwallet", "closed", false), None);
        assert_eq!(on_focus("btcwallet"), None);
        assert_eq!(on_plugin_load("btcwallet"), None);
    }

    #[test]
    fn focusing_a_tab_dismisses_a_notification_but_not_a_running_agent() {
        assert_eq!(on_focus("● btcwallet"), Some("  btcwallet".into()));
        assert_eq!(on_focus("✓ btcwallet"), Some("  btcwallet".into()));
        assert_eq!(on_focus("◐ btcwallet"), None, "a working agent is status, not a notification");
        assert_eq!(on_focus("  btcwallet"), None);
    }

    #[test]
    fn a_fresh_plugin_instance_retires_spinners_left_by_a_resurrected_session() {
        assert_eq!(on_plugin_load("◐ btcwallet"), Some("  btcwallet".into()));
        // An unanswered notification from before the session died still stands.
        assert_eq!(on_plugin_load("● btcwallet"), None);
        assert_eq!(on_plugin_load("✓ btcwallet"), None);
    }

    #[test]
    fn only_the_states_zwm_attn_records_are_marks() {
        assert_eq!(Mark::from_signal("working"), Some(Mark::Working));
        assert_eq!(Mark::from_signal("waiting"), Some(Mark::Waiting));
        assert_eq!(Mark::from_signal("done"), Some(Mark::Done));
        assert_eq!(Mark::from_signal("closed"), Some(Mark::Cleared));
        assert_eq!(Mark::from_signal("focus"), None, "focus is not a state; the plugin handles it first");
    }

    #[test]
    fn a_glyph_is_drawn_for_a_state_and_not_for_a_blank_slot() {
        assert_eq!(Mark::Working.glyph(), Some('◐'));
        assert_eq!(Mark::Waiting.glyph(), Some('●'));
        assert_eq!(Mark::Done.glyph(), Some('✓'));
        assert_eq!(Mark::Cleared.glyph(), None);
        assert_eq!(Mark::Unmarked.glyph(), None);
    }

    #[test]
    fn a_state_marker_leads_with_the_glyph_the_bar_draws_for_it() {
        // The dashboard reads the raw name and the bar redraws the glyph itself;
        // the two must agree on what a marked tab looks like.
        for mark in [Mark::Working, Mark::Waiting, Mark::Done] {
            let glyph = mark.glyph().expect("a state has a glyph");
            assert_eq!(mark.prefix(), format!("{glyph} "), "{mark:?}");
        }
    }

    #[test]
    fn the_spinner_starts_on_the_glyph_the_marker_itself_carries() {
        // So a bar that has not ticked yet, and the raw tab name in
        // `query-tab-names`, show the same character.
        assert_eq!(Mark::Working.glyph(), Some(SPINNER[0]));
    }
}
