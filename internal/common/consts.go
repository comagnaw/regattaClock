package common

const (
	AppTitle    = "Regatta Clock"
	AppBundleID = "com.github.comagnaw.regattaClock"

	PrefRegattaDir  = "RegattaDir"
	PrefTheme       = "Theme"
	PrefLight       = "Light"
	PrefDark        = "Dark"
	PrefDebug       = "Debug"
	PrefLogging     = "Logging"
	PrefStorageMode = "StorageMode"
	PrefNTPServers  = "NTPServers"

	// PrefLastPersonaID - the persona ID chosen on the previous run. Only used
	// to offer the Regatta Director a "resume" shortcut on the picker; timers
	// always re-pick.
	PrefLastPersonaID = "LastPersonaID"

	// PrefPersonaConfigFile - absolute path to an optional deployment JSON
	// (internal/personacfg) that pins this host to a persona (skipping the
	// picker) and/or replaces the built-in challenge codes. Chosen on the
	// Configuration screen; blank means the normal persona picker.
	PrefPersonaConfigFile = "PersonaConfigFile"

	// StorageModeCloud / StorageModeSMB are the PrefStorageMode values. They
	// must stay equal to watcher.ModeCloud / watcher.ModeSMB (asserted by a test
	// in the watcher package); common stays a leaf and cannot import watcher.
	StorageModeCloud = "cloud"
	StorageModeSMB   = "smb"

	RegattaDataDir = "regattaData"

	// LegacyDataFile - the pre-persona single-blob schedule+results file at
	// regattaData/data.json. Read once by the migration into
	// director/regattaSchedule.json, then renamed aside.
	LegacyDataFile = "data.json"

	// LogsDir - subtree of RegattaDataDir holding per-persona event logs
	// (internal/applog). The persona phases nest this as logs/<team>/<role>-<host>.log;
	// today the single operator writes one flat file here.
	LogsDir = "logs"

	ResultsSheetName = "Results"

	EmptyString = ""

	ZeroTime = "00:00.0"

	Padding             = "                                              "
	HiddenFileFormatter = ".%s"
	ClockFormatter      = "%02d:%02d.%d"

	RaceDisqualification = "DQ"
	RaceDidNotFinish     = "DNF"
	RaceDidNotStart      = "DNS"

	RaceOrderOfFinish = "OOF"
	RacePlace         = "Place"
	RaceSplit         = "Split"
	RaceTime          = "Time"
	RaceSchool        = "School"

	// ClockTimingForFormat / ClockResultsZoneLabel - the accent-band headers that
	// split the race clock into its operator-controls zone and its results
	// readout, matching the race tree's column-header band. The Timing band reads
	// as one centred phrase - the regattaClock wordmark then this text - so it
	// doubles as the window's heading. Arg: the race title.
	ClockTimingForFormat  = "timing for %s"
	ClockResultsZoneLabel = "Results"

	// Compare Secondary - the primary finish timer's read-only side-by-side view
	// of the secondary team's committed result for the same race (compare.go).
	// The button toggles the pane; the pane never edits the SFT's data.
	CompareSecondaryButtonText = "Compare Secondary"
	CompareSecondaryHideText   = "Hide Secondary"
	CompareSecondaryBandFormat = "Secondary timer — %s" // arg: race title
	// CompareSkewNoteFormat - shown in the compare pane when the two timers'
	// machine clocks disagree by more than timesync.SkewWarnThreshold. Args:
	// primary machine, secondary machine, offset delta.
	CompareSkewNoteFormat = "%s and %s clocks differ by %s — the times below may be off by that much. Reconcile with care."

	RefereeButtonText   = "Referee Approval"
	RefereeApproveTitle = "Referee Approval - Race %d"

	EditPlaceTitle = "Edit Place %d"

	SaveSkippedTitle   = "Save Skipped"
	SaveSkippedMessage = "Regatta data could not be saved, so this session will not be restored on the next start.\n\n%s"

	BannerResourceName = "RegattaClockBanner.svg"

	// Regatta Director setup view (internal/regatta regatta.go / config.go). Two
	// steps on one screen, each filling in with a check mark, the parsed regatta
	// details and the chosen path; Start Regatta ungates once both are done. The
	// welcome banner carries the app name, so the steps need no title above them.
	SetupExcelStepText       = "Step 1  —  Load the Excel workbook that holds your regatta schedule."
	SetupExcelDoneText       = "Step 1  —  Regatta schedule loaded  ✓"
	SetupExcelFilePathLabel  = "Loaded from:"
	SetupSaveDirStepText     = "Step 2  —  Choose the folder where regatta data is saved."
	SetupSaveDirDoneText     = "Step 2  —  Save location set  ✓"
	SetupSaveDirPathLabel    = "Regatta data will be saved to:"
	SetupChangeDirButtonText = "Change…"
	SetRegattaDirTitle       = "Set regatta directory"
	SetupNeedSaveDirMessage  = "Choose where regatta data is saved before starting the regatta."

	// ConfirmImportedRegattaMessage - shown to the Regatta Director after an
	// Excel import, before the schedule is written. Deny returns to file
	// selection. Args: name, date, scheduled race count.
	ConfirmImportedRegattaMessage = "%s\n%s\nScheduled races: %d\n\nUse this as the regatta schedule?"
	// ResumeDirectorFormat - picker shortcut label; arg is the regatta name.
	ResumeDirectorFormat = "Resume as Regatta Director — %s"

	// Reload Schedule + the RegattaKey-mismatch guard (persona-plan.md 3b,
	// "A different regatta is not a schedule change"). internal/regatta
	// schedule_guard.go / loader.go / menu.go.
	ReloadScheduleTitle        = "Reload Schedule"
	NoOriginRecordedMessage    = "No source workbook is recorded for this regatta. Use \"Load Regatta Data\" to pick one."
	ReloadFailedMessage        = "Could not re-read the regatta workbook"
	ExistingScheduleUnreadable = "The schedule already in %s could not be read. Resolve it before importing.\n\n%v"
	DifferentRegattaTitle      = "Different regatta"
	// Args: on-disk name, on-disk date, workbook name, workbook date.
	DifferentRegattaBlockedMessage = "This workbook describes a different regatta:\n\n  on disk:  %s  (%s)\n  workbook: %s  (%s)\n\nThis folder already holds timing data for the regatta on disk. Point the Regatta Director at a fresh regatta folder, or archive this folder's director/ and timing/ data yourself, then import again."
	// Args: on-disk name, on-disk date, workbook name, workbook date, old RegattaKey.
	DifferentRegattaReplaceMessage = "This workbook describes a different regatta:\n\n  on disk:  %s  (%s)\n  workbook: %s  (%s)\n\nNo timing data has been recorded here yet. Replace the schedule? The current one is kept as regattaSchedule.%s.json."

	// Origin-refresh banner (persona-plan.md 3b step 5). The RD polls the source
	// workbook; a change to the schedule *content* (not just the file) raises
	// this. Arg: a short summary of what changed.
	ApplyButtonText           = "Apply"
	OriginChangedBannerFormat = "The regatta workbook has changed: %s. Apply to publish it to every timer, or dismiss."
	OriginUnchangedMessage    = "The workbook has not changed the schedule."

	NumScheduledRacesTitle = "Scheduled Races: %d"
	ScheduledRacesTile     = "Scheduled Races"

	// TreeRegattaLabel / TreeDateLabel - the race-tree details panel shows the
	// regatta name and date in the same "Key: Value" form as PersonaHeaderFormat
	// and NumScheduledRacesTitle, so the four fields read as one 2x2 block.
	TreeRegattaLabel = "Regatta: %s"
	TreeDateLabel    = "Date: %s"

	// PersonaHeaderFormat labels the race-tree header with the operator's role;
	// WindowTitleFormat puts it in the OS title bar next to the app name.
	PersonaHeaderFormat   = "Role: %s"
	WindowTitleFormat     = "%s — %s"
	ConfigTitle           = "Configuration"
	LoadDataTitle         = "Load Regatta Data"
	CreateLaneImagesTitle = "Create Lane Images"
	// VersionTitle is both the menu label and the title of the build-info window.
	VersionTitle = "Version"

	// Race-tree column headers (internal/regatta races.go / timer_races.go). The
	// race column reuses ScheduledRacesTile.
	ColStartTime   = "Start Time"
	ColStatus      = "Status"
	ColRestarts    = "Restarts"
	ColWinningTime = "Winning Time"

	// Role-aware timer race tree (internal/regatta timer_races.go / start_timing.go).
	StartTimeButtonText      = "Start Time"
	ClearButtonText          = "Clear"
	RestoreButtonText        = "Restore"
	NoStartTimeText          = "—"
	WaitingForStartText      = "awaiting start"          // FT race-tree Start Time cell before the peer start lands (fits the start-time column)
	StartNotCollectedText    = "no start time"           // FT race-tree Start Time cell once a result is saved/approved and no start was recorded
	WaitingForStartTimeText  = "waiting for start time…" // FT clock winning-time placeholder until the ST start lands
	RaceSavedText            = "saved"
	RaceApprovedText         = "approved"
	StartTimeDisplayLayout   = "15:04:05.0" // wall clock with tenths, as StartRecord.Display
	ClearStartTitle          = "Clear start time"
	ClearStartMessage        = "Clear the recorded start for race %d?"
	RestoreStartTitle        = "Restore start time"
	RestoreStartPlainMessage = "Restore the previously collected start time %s for race %d?"
	RestoreStartMessage      = "Replace the current start time %s with the previously collected %s for race %d?"
	WritesBlockedMessage     = "Recording is blocked because a timing file could not be read at startup. Resolve the file set aside for recovery and restart."

	// Race-progress status, one vocabulary across the ST, FT and RD race trees:
	// FirstFinishAt set -> RaceInProgressText, a winning time saved ->
	// RaceSavedText (above), referee-approved -> RaceApprovedText (above).
	RaceInProgressText = "timing in progress"

	// Schedule-conflict notices (persona-plan.md 3c). A schedule change that
	// touches a race with timing (or an open clock) never rewrites start.json /
	// finish.json - it only refreshes labels and raises these.
	ScheduleConflictMark = "⚠ " // row-title prefix while a conflict is unacknowledged

	// StaleLaneMapMark / StaleLaneMapLegend (persona-plan.md 3c item 4). A
	// persistent row-title mark on the RD and FT trees when a committed
	// RaceResult's stored LaneMapHash no longer matches the live schedule's -
	// i.e. results were entered against an earlier lane map. Survives a restart.
	StaleLaneMapMark             = "† "
	StaleLaneMapLegend           = "†  results were committed against an earlier lane map — reopen the race to review, then Save"
	ScheduleConflictStartBanner  = "Schedule updated for race %s - lane assignments changed. Recorded start times are unaffected."
	ScheduleConflictFinishBanner = "Schedule changed for race %s (scratch / lane reassignment). Review order of finish and results."
	ClockScheduleNoticeFormat    = "Schedule changed for race %d - lane labels refreshed. Lap times and order of finish are unchanged; review the highlighted lanes."

	// ClockSkewBannerFormat - persona-plan.md 2.1: a persistent, dismissible
	// banner shown on the FT clock when the two machines' measured offsets
	// disagree by more than timesync.SkewWarnThreshold. Args: FT machine, ST
	// machine, offset delta, FT offset, ST offset.
	ClockSkewBannerFormat = "Clock skew: %s and %s clocks differ by %s (offsets %s and %s). Winning times may be off by that much until the machines agree."
	DismissButtonText     = "Dismiss"

	// DirectorSkewBannerFormat - persona-plan.md 2.1 skew banner on the RD tree,
	// comparing the offsets stamped on the primary team's timing-file envelopes.
	// Args: machine A, machine B, offset delta.
	DirectorSkewBannerFormat = "Clock skew: %s and %s differ by %s. Primary-team race times may be off by that much."

	// DirectorStaleBannerFormat - persona-plan.md 9 staleness indicator: no
	// timing file has been written for a while. Arg: age of the freshest write.
	DirectorStaleBannerFormat = "No timing updates in %s. The regatta may have stalled, or a timer's machine is offline."

	// Winning-time helper note under the FT clock's Winning Time field
	// (persona-plan.md 2.1). The derived value only pre-fills; the referee's
	// time always overrides. These say where the number came from, or why there
	// is none.
	WinningTimeDerivedNote  = "Auto-filled from the start timer. The referee's official time overrides this."
	WinningTimeWaitingNote  = "Waiting for the start timer to record a start time for this race…"
	WinningTimeTinyNote     = "Auto-filled %s - the start time and first finish are seconds apart. Verify the start timer or enter the referee's time."
	WinningTimeStaleNote    = "Auto winning time skipped: the recorded start time is about %s old. Enter the referee's time."
	WinningTimeNegativeNote = "Auto winning time skipped: the start time is %s later than the first finish (clock skew?). Enter the referee's time."

	// FT clock commit-status line, under the approval panel. A race is Pending
	// until it is persisted: the primary FT reaches Approved via Referee
	// Approval, the secondary FT reaches Saved via Save and Close. The primary
	// FT's Close button stays disabled until the line leaves Pending. The
	// non-Pending lines are "<state> at <time> by <host>"; args are the local
	// timestamp then the writing machine's hostname.
	CommitStatusPending        = "Pending"
	CommitStatusSavedFormat    = "Saved on %s by %s"
	CommitStatusApprovedFormat = "Approved on %s by %s"
	CommitStatusTimeFormat     = "Mon, 02 Jan 2006 15:04:05 MST" // time.RFC1123
	CommitStatusUnknownHost    = "unknown host"

	// Persona picker (internal/regatta persona_startup.go). The Welcome screen
	// groups personas into tabs; pressing a persona's button asks for its
	// challenge code in a small dialog. Media/Developer are disabled placeholders
	// for personas that do not exist yet.
	PersonaPickerPrompt         = "Choose your persona."
	PersonaChallengeTitle       = "Challenge for %s"
	ContinueButtonText          = "Continue"
	ChallengeFieldLabel         = "Challenge code:"
	PersonaTabTimers            = "Timers"
	PersonaTabMedia             = "Media"
	PersonaTabAdmins            = "Admins"
	PersonaPlaceholderNote      = "Greyed-out personas are planned for a future release."
	PersonaSocialMediaLabel     = "Social Media"
	PersonaStreamingLabel       = "Streaming"
	PersonaRegisterResultsLabel = "Register Results"
	PersonaDeveloperLabel       = "Developer"
	SelectRegattaFolderTitle    = "Select the shared regatta folder"
	UseThisFolderText           = "Use This Folder"
	ChallengeMismatchMessage    = "That challenge code does not match the selected persona."
	NotRegattaDataDirMessage    = "Choose the shared regatta folder - the one that contains a regattaData folder (or regattaData itself)."
	ScheduleUnreadableMessage   = "Could not read the regatta schedule in that directory"
	ConfirmRegattaTitle         = "Confirm regatta"
	ConfirmRegattaMessage       = "%s\n%s\nScheduled races: %d\n\nTime this regatta?"

	// Deployment persona config (internal/personacfg + internal/regatta
	// persona_config.go). An organisation points the app at a JSON file on the
	// Configuration screen; it can pin a host to a persona (skipping the picker)
	// and/or replace the challenge codes. Every load failure is non-fatal - the
	// picker is the fallback - and "Switch Persona" on every menu re-opens it.
	PersonaConfigRowLabel                 = "Persona Config:"
	PersonaConfigChangeButtonText         = "Change Persona Config"
	PersonaConfigLoadFailedFormat         = "The deployment persona config could not be loaded, so the normal persona picker is being used.\n\n%s"
	PersonaConfigLoadedTitle              = "Persona config loaded"
	PersonaConfigLoadedFormat             = "Loaded - %d host assignment(s), %d challenge override(s).\n\nA host assignment applies at the next launch or via Switch Persona."
	PersonaConfigInvalidTitle             = "Persona config not loaded"
	SwitchPersonaMenuLabel                = "Switch Persona..."
	SwitchPersonaConfirmTitle             = "Switch persona?"
	SwitchPersonaConfirmMessage           = "The current session will close and the persona picker will re-open. Open race-clock windows are left as they are."
	AssignedPersonaBannerFormat           = "You are set up as %s on this computer."
	AssignedPersonaSelectFolderButtonText = "Select regatta folder"
	AssignedPersonaSwitchNote             = "Wrong role for this machine? Use the " + AppTitle + " menu -> Switch Persona to choose a different one."

	// Past-regatta gate (internal/regatta date_guard.go). Shown to a timer when
	// the schedule's date is before the host's current local date - the
	// "already-run regatta" mistake. An empty or unparseable date skips it.
	// PastRegattaMessage args: regatta name, regatta date, today.
	PastRegattaTitle         = "Regatta date has passed"
	PastRegattaMessage       = "\"%s\" was scheduled for %s.\nToday is %s.\n\nTiming data is the permanent record for a regatta. Loading it now means recording times against an event that has already run.\n\nLoad it anyway?"
	RegattaDateDisplayLayout = "Monday, January 2, 2006"

	CorruptTimingFileTitle   = "Timing file could not be read"
	CorruptTimingFileMessage = "%s could not be parsed and has been copied aside as %s. Recording is blocked until this is resolved so a day's data is not overwritten.\n\n%s"

	ApproveButtonText       = "Approve"
	CancelButtonText        = "Cancel"
	CloseButtonText         = "Close"
	ExitButtonText          = "Exit"
	LapButtonText           = "Lap (F4)"
	LoadButtonText          = "Load"
	LoadExcelButtonText     = "Load Excel File"
	SetRegattaDirButtonText = "Set Regatta Directory"
	StartRegattaButtonText  = "Start Regatta"
	SaveButtonText          = "Save"
	SaveAndCloseButtonText  = "Save and Close"
	ShowWindowText          = "Show Window"
	StartButtonText         = "Start (F2)"
	StopButtonText          = "Stop"
	TimeRaceButtonText      = "Time Race"
	WinningTimeInputText    = "Winning Time:"
)

// RegattaFileExtensions - spreadsheet extensions the reader can parse, shared by
// the file dialog filter so it cannot drift from what the loader accepts.
var RegattaFileExtensions = []string{".xlsx", ".xlsm"}

// PersonaConfigExtensions - the file-dialog filter for the optional deployment
// persona config picked on the Configuration screen.
var PersonaConfigExtensions = []string{".json"}
