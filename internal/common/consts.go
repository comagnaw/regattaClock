package common

const (
	AppTitle    = "Regatta Clock"
	AppBundleID = "com.github.comagnaw.regattaClock"

	PrefRegattaDir = "RegattaDir"
	PrefTheme      = "Theme"
	PrefLight      = "Light"
	PrefDark       = "Dark"
	PrefDebug      = "Debug"
	PrefLogging    = "Logging"

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

	NumScheduledRacesTitle = "Scheduled Races: %d"
	ScheduledRacesTile     = "Scheduled Races"
	ConfigTitle            = "Configuration"
	LoadDataTitle          = "Load Regatta Data"
	CreateLaneImagesTitle  = "Create Lane Images"

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
