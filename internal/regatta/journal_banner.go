package regatta

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/journal"
)

// journalStatusBanner builds the (initially hidden) local write-ahead journal
// status banner for a timer session (persona-plan.md section 13): an
// actionBanner reused from the RD's origin-change banner, with "Retry Now"
// wired to journal.Manager.Nudge. showRaceTree calls this while assembling
// the header, alongside scheduleBannerWidget.
//
// Constructing the journal.Manager here, via journal.For, is what lets a
// finish/start timer's own crash-recovery check (hydrateOwnStart /
// hydrateOwnFinish, which also call journal.For) and this banner share one
// instance - by the time startSession reaches showRaceTree, that Manager
// already exists, so this call only ever returns the memoized one.
func (r *Regatta) journalStatusBanner() fyne.CanvasObject {
	if r.session.Role != persona.RoleStart && r.session.Role != persona.RoleFinish {
		// Director and Awards never own a timing file (persona.Session.WritePath
		// is empty for both), so there is no journal.Manager for this session.
		return container.NewWithoutLayout()
	}

	j, err := journal.For(r.session, r.regattaKey)
	if err != nil {
		applog.Warn("write-ahead journal unavailable; sync status banner disabled",
			"component", "race_tree", "err", err)
		return container.NewWithoutLayout()
	}

	b := newActionBanner(common.RetryNowButtonText, j.Nudge, func() { r.journalBanner.hide() })
	r.journalBanner = b
	r.stopJournalStatus = j.Subscribe(func(s journal.Status) {
		fyne.Do(func() { r.refreshJournalBanner(s) })
	})
	return b.root
}

// refreshJournalBanner shows or hides journalBanner from a journal.Manager
// Status. Queued is deliberately treated the same as Synced (hidden): a
// healthy shared path resolves a queued write in milliseconds, so showing
// anything for that state would only flicker. StateLocalWriteFailed - the
// local staging write itself failing - is not surfaced here; that already
// reaches the operator as a dialog.ShowError at the call site (persist.go,
// start_timing.go), a rarer and more serious condition than an ambient
// banner fits.
func (r *Regatta) refreshJournalBanner(s journal.Status) {
	if r.journalBanner == nil {
		return
	}
	if s.State != journal.StateRetrying {
		r.journalBanner.hide()
		return
	}
	elapsed := time.Since(s.Since).Round(time.Second)
	r.journalBanner.show(fmt.Sprintf(common.JournalRetryingBannerFormat, elapsed))
}
