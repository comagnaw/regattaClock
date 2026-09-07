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

	RefereeButtonText   = "Referee Approval"
	RefereeApproveTitle = "Referee Approval - Race %d"

	EditPlaceTitle = "Edit Place %d"

	SaveSkippedTitle   = "Save Skipped"
	SaveSkippedMessage = "Regatta data could not be saved, so this session will not be restored on the next start.\n\n%s"

	BannerResourceName = "RegattaClockBannerSmall.png"

	// The welcome banner carries the app name, so the steps need no title above them
	WelcomeSetDirText   = "1. Set the directory for loading and saving regatta data."
	WelcomeLoadFileText = "2. Load the Excel file holding your regatta schedule."

	// ConfirmImportedRegattaMessage - shown to the Regatta Director after an
	// Excel import, before the schedule is written. Deny returns to file
	// selection. Args: name, date, scheduled race count.
	ConfirmImportedRegattaMessage = "%s\n%s\nScheduled races: %d\n\nUse this as the regatta schedule?"
	// ResumeDirectorFormat - picker shortcut label; arg is the regatta name.
	ResumeDirectorFormat = "Resume as Regatta Director — %s"

	NumScheduledRacesTitle = "Scheduled Races: %d"
	ScheduledRacesTile     = "Scheduled Races"

	// PersonaHeaderFormat labels the race-tree header with the operator's role;
	// WindowTitleFormat puts it in the OS title bar next to the app name.
	PersonaHeaderFormat   = "Role: %s"
	WindowTitleFormat     = "%s — %s"
	ConfigTitle           = "Configuration"
	LoadDataTitle         = "Load Regatta Data"
	CreateLaneImagesTitle = "Create Lane Images"

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
	WaitingForStartText      = "waiting for start…"
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
	RaceLockedTimingText     = "timing in progress"
	RaceLockedResultsText    = "results recorded"

	// Schedule-conflict notices (persona-plan.md 3c). A schedule change that
	// touches a race with timing (or an open clock) never rewrites start.json /
	// finish.json - it only refreshes labels and raises these.
	ScheduleConflictMark         = "⚠ " // row-title prefix while a conflict is unacknowledged
	ScheduleConflictStartBanner  = "Schedule updated for race %s - lane assignments changed. Recorded start times are unaffected."
	ScheduleConflictFinishBanner = "Schedule changed for race %s (scratch / lane reassignment). Review order of finish and results."
	ClockScheduleNoticeFormat    = "Schedule changed for race %d - lane labels refreshed. Lap times and order of finish are unchanged; review the highlighted lanes."

	// ClockSkewBannerFormat - persona-plan.md 2.1: a persistent, dismissible
	// banner shown on the FT clock when the two machines' measured offsets
	// disagree by more than timesync.SkewWarnThreshold. Args: FT machine, ST
	// machine, offset delta, FT offset, ST offset.
	ClockSkewBannerFormat = "Clock skew: %s and %s clocks differ by %s (offsets %s and %s). Winning times may be off by that much until the machines agree."
	DismissButtonText     = "Dismiss"

	// Regatta Director progress tree (internal/regatta director_tree.go). A cell
	// whose value fell back to the secondary team is suffixed with
	// SecondaryValueMark and explained by SecondaryValueLegend under the header.
	SecondaryValueMark   = " ·2nd"
	SecondaryValueLegend = "·2nd  value from the secondary team"

	// DirectorSkewBannerFormat - persona-plan.md 2.1 skew banner on the RD tree,
	// comparing the offsets stamped on the four timing files' envelopes. Args:
	// machine A, machine B, offset delta.
	DirectorSkewBannerFormat = "Clock skew: %s and %s differ by %s. Race times combining both teams may be off by that much."

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

	// Timer persona startup flow (internal/regatta persona_startup.go).
	PersonaPickerPrompt           = "Choose your persona and enter its challenge code."
	ChallengeFieldLabel           = "Challenge code:"
	SelectRegattaFolderButtonText = "Select Regatta Folder"
	SelectRegattaFolderTitle      = "Select the shared regatta folder"
	UseThisFolderText             = "Use This Folder"
	NoPersonaSelectedMessage      = "Select a persona before continuing."
	ChallengeMismatchMessage      = "That challenge code does not match the selected persona."
	NotRegattaDataDirMessage      = "Choose the shared regatta folder - the one that contains a regattaData folder (or regattaData itself)."
	ScheduleUnreadableMessage     = "Could not read the regatta schedule in that directory"
	ConfirmRegattaTitle           = "Confirm regatta"
	ConfirmRegattaMessage         = "%s\n%s\nScheduled races: %d\n\nTime this regatta?"
	CorruptTimingFileTitle        = "Timing file could not be read"
	CorruptTimingFileMessage      = "%s could not be parsed and has been copied aside as %s. Recording is blocked until this is resolved so a day's data is not overwritten.\n\n%s"

	ApproveButtonText       = "Approve"
	CancelButtonText        = "Cancel"
	CloseButtonText         = "Close"
	ExitButtonText          = "Exit"
	LapButtonText           = "Lap (F4)"
	LoadButtonText          = "Load"
	LoadExcelButtonText     = "Load Excel File"
	SetRegattaDirButtonText = "Set Regatta Directory"
	SaveButtonText          = "Save"
	ShowWindowText          = "Show Window"
	StartButtonText         = "Start (F2)"
	StopButtonText          = "Stop"
	TimeRaceButtonText      = "Time Race"
	WinningTimeInputText    = "Winning Time:"
)

// RegattaFileExtensions - spreadsheet extensions the reader can parse, shared by
// the file dialog filter so it cannot drift from what the loader accepts.
var RegattaFileExtensions = []string{".xlsx", ".xlsm"}
