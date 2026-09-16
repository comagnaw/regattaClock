# Social Media (SOM)

A **sidecar capability**, not a standalone persona — formalizes the
already-sketched `social-post` capability in
[sidecar-personas.md](../sidecar-personas.md) and designs the automated
X-posting increment that doc only placeholder'd ("Social Post — automated
X / social posting (sidecar increment 2). Needs: OAuth2, media upload,
rate-limit handling"). SOM **is** the persona-facing name for that
capability attached to a Lead session — there is no new `persona.Role` /
`Definition` / challenge code.

This resolves "Social Post" (`docs/features/TODO.md`) and the "social-text
publisher" from
[future-result-driven-persona.md](../future-result-driven-persona.md) —
both are SOM; SOM is text-only. That doc also sketched a separate
"social-image publisher" concept (PNG output) — the author has since said
that is not an expected necessity, so it is not carried forward as a
placeholder anywhere; a PNG-rendering need, if one arises, is the
still-unscoped Streamer (STM) persona's job (see "Does not," above).

## Decisions made (author, 2026-09-16)

- **Shape:** sidecar capability attached to a Lead, not a standalone
  persona.
- **Host scope:** narrower than `sidecar-personas.md`'s general
  `isLead()` guard — only the **Finish Timers** (PFT/SFT) may open the
  `social-post` sidecar, matching how Results Publisher was scoped.
  Nothing to post exists before a race is approved, which only happens at
  the finish line.
- **Text format / shared package:** already satisfied. `internal/publish`'s
  Phase 0 sketch (`PublishableRace`, `BuildView`, `RenderText`) is the
  "standard results format" the author asked SOM to share — specifically
  with the still-unscoped **Streamer (STM)** persona, not Results
  Publisher (an earlier pass at this doc incorrectly linked it to REP;
  the author corrected that). SOM posts `RenderText(pr)`'s plain-text
  table as-is (plus any extra text the operator adds); STM's job, per the
  author, is to take that **same shared text** and transform it into a
  PNG — the same underlying per-race, per-lane joined data, two different
  renderers. SOM depends on `internal/publish` as-is, no new package
  designed here; STM's text-to-PNG transform is not designed further in
  this doc either, since STM itself remains unscoped, but the dependency
  target (`internal/publish`) is confirmed shared between the two.
- **X API authentication:** unresolved, and deliberately **not**
  researched as part of this doc — flagged as its own prerequisite
  investigation (OAuth flow, developer account/API tier, cost, rate
  limits), the same posture this project took with RegattaCentral before
  attempting to publish there
  ([heatsheet-rc-pivot-investigation.md](../heatsheet-rc-pivot-investigation.md)).
  Nothing in this doc should be read as having resolved that unknown.

## Does / Does not / Entry / Constraint

- **Does:** Add a **Social** publish panel to a Finish Timer's session
  (`common.SidecarSocialLabel`, already reserved in
  `sidecar-personas.md`'s Phase 0d menu sketch), showing one row per
  approved race with clear approval visibility (timestamp from
  `PublishableRace.ApprovedAt`) and a **Publish** button, enabled once
  that race is approved. Selecting Publish opens a pop-up window
  pre-filled with `publish.RenderText(pr)` (the plaintext table) in a
  monospace, **editable** text area, sized to comfortably show a full
  race's results without scrolling for a typical field size, with room
  for the operator to add extra plain text (e.g. a comment, a hashtag)
  above or below the table. The window has **Publish** and **Close**
  buttons.
- **Does not:** Publish an unapproved race. Auto-post without the
  operator seeing and being able to edit the exact text first — there is
  no "publish directly from the row" shortcut. Post or attach an image —
  SOM is **plain text only**; it never touches the image/PNG path
  (`internal/exporter`'s `RenderResult`, Phase 0b) — see Streamer (STM)
  below. Require Unicode/emoji input or
  rendering — plain ASCII-range text is sufficient; nothing in this
  design assumes emoji support anywhere in the edit box, the character
  count, or the eventual X post. Attach to a Start Timer or the
  Director. The image path is the still-unscoped Streamer (STM)
  persona's job, per the author — STM takes the same shared text and
  renders it as a PNG; not designed further here.
- **Entry:** No new entry point or challenge — it's a capability opened
  from an already-running PFT/SFT session's menu
  (`publishMenu()`, Phase 0d), same mechanism as the render-only Phase 0
  version already sketched.
- **Constraint:** Only one sidecar open at a time (`Regatta.sidecar`,
  Phase 0c's existing "at most one" invariant) — SOM and any other
  capability (e.g. a future Register-Results-successor) cannot both be
  open simultaneously on the same session, unchanged from the existing
  design.

## The publish-window workflow, in detail

This is the part `sidecar-personas.md` left as a one-line placeholder;
the rest of this doc's design reuses that doc's Phase 0 pieces as-is.

1. **Publish** (row) → open the pop-up, pre-filled with `RenderText(pr)`,
   editable, Publish/Close buttons.
2. **Publish** (pop-up) → send the current text to X.
   - On success: close the window; mark that race `published[n] =
     Revision(pr)` (reusing Phase 0c's existing `PrefPublishedFormat`
     preference and `published map[int]string` field verbatim — this is
     exactly the re-publish-detection mechanism Phase 0 already built for
     Copy/Save; posting to X is just a new way to reach the same mark);
     the row's status updates from `Approved` to **Approved/Published**.
   - On failure: the window stays open, gains a warning banner naming the
     failure, and the operator may edit and retry — no auto-retry, no
     silent drop.
3. **Close** (pop-up, at any point) → discard the window and any edits.
   Nothing is saved, nothing is published. The row's status is
   unaffected.
4. **Re-publish**: if the underlying result changes after a successful
   publish (`Revision(pr)` no longer matches `published[n]`), the row
   re-flags exactly as Phase 0's Copy/Save re-publish mark already does —
   same mechanism, same visual treatment, now also triggered by a
   successful X post.

## Existing-code reuse analysis

- **`internal/publish`** (Phase 0a, already fully sketched) —
  `PublishableRace`, `Row`, `BuildView`, `Revision`, `RenderText` are
  reused as-is. SOM adds no new fields to `PublishableRace`; the pop-up
  needs nothing `RenderText`'s output doesn't already provide.
- **Sidecar lifecycle** (Phase 0c, `internal/regatta/sidecar.go`) — reuse
  `openSidecar`/`closeSidecar`, the `sidecar` struct, and the `published
  map[int]string` + `PrefPublishedFormat` preference verbatim. The only
  change to this existing sketch: `openSidecar("social-post")`
  specifically also checks `r.session.Role == persona.RoleFinish` (Finish
  Timers only), layered on top of the existing, broader `r.isLead()`
  guard — not a replacement for it, since other capabilities (a future
  Register-Results-successor) may still want the general Lead scope.
- **Menu wiring** (Phase 0d) — `common.SidecarSocialLabel` is already a
  named constant in the sketch; SOM's Publish button is new UI inside the
  sidecar window Phase 0c already opens, not a new menu entry.
- **New work, not reuse**: the pop-up window itself (editable text area,
  Publish/Close, warning banner, retry) — Phase 0 only sketched Copy
  (clipboard) and Save image (file), both fire-and-forget with no
  response to handle. A real network call with success/failure states is
  new UI and control flow. The actual X API client (auth, the post call
  itself) is also entirely new — no `internal/social` or similar package
  exists yet, and per the decision above, its authentication shape is not
  designed here.
- **Generic multi-platform interface**: `sidecar-personas.md`'s
  `internal/publish` `Target` interface
  (`type Target interface { Name() string; Publish(ctx, PublishableRace)
  error }`, sketched for the RegattaCentral push and noted as reusable
  for "room for more" capability targets) is the natural shape for an
  X-specific implementation, keeping the pop-up's Publish button
  target-agnostic. Not designed further here — worth confirming this
  interface's exact shape once the RC-facing implementation (or its
  replacement, given Results Publisher's own resolution) exists as a
  second real example, so X isn't the interface's only implementation to
  design against.

## High-level implementation plan

1. **Land Phase 0** as `sidecar-personas.md` already specifies
   (`internal/publish`, the image-render addition, `sidecar.go`, the menu
   wiring) — SOM has nothing to attach to otherwise. Not new work this
   doc introduces; a prerequisite already fully designed elsewhere.
2. **Scope the X API investigation** — its own short research pass
   (auth flow, account/API tier, cost, rate limits) before any client
   code, mirroring `heatsheet-rc-pivot-investigation.md`'s approach.
3. **X client**: an `internal/social` (or similar) package implementing
   `publish.Target` for X, once the investigation above resolves what
   that requires.
4. **Pop-up window**: editable monospace text area pre-filled from
   `RenderText`, Publish/Close buttons, a warning-banner state for a
   failed post, wired to reuse Phase 0c's `published`/
   `PrefPublishedFormat` tracking on success.
5. **Row status**: extend the sidecar row rendering to show
   **Approved/Published** when `published[n] == Revision(pr)`, matching
   Phase 0's existing re-publish-flag visual treatment for the "stale"
   case.
6. **Docs** (once built): update `sidecar-personas.md`'s own status once
   its Phase 0 + this increment both ship (it already anticipates
   updating its own "status" line per Phase 0e); update
   `docs/features/TODO.md`; delete this file.

## Dependencies and sequencing

- **Hard dependency**: Phase 0 (`internal/publish`, `sidecar.go`, menu
  wiring) from `sidecar-personas.md` — not yet built. This is shared
  infrastructure Results Publisher's own resolution did **not** need
  (REP became a PFT-native feature, bypassing the sidecar seam entirely),
  so Phase 0 remains unbuilt regardless of REP's resolution — SOM is what
  actually requires it.
- **External unknown, not an internal blocker**: X's API authentication
  requirements — flagged, not resolved, per the decision above.
- **No conflict** with Results Publisher: different destination, different
  attachment model. REP is a native PFT feature, not a sidecar, so
  `sidecar-personas.md`'s "at most one sidecar" invariant never applies
  to it — REP's Publish button and an open Social sidecar panel can
  coexist on the same PFT session without any special-casing.

**No architectural blockers beyond Phase 0 needing to land first**, and
Phase 0 itself has none. As with the other proposals in this directory,
the live process constraint as of this writing is `develop`'s
feature-freeze (see `AGENTS.md`) — implementation waits for that to lift.
