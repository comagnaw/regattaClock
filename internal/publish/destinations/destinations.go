// Package destinations is the registry of results-publish destination kinds
// (publish-destinations.md): what each stored kind ID means, whether it
// publishes into a folder, and whether it is built yet. It sits apart from
// internal/publish because the implementations it names import that package.
package destinations

import (
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/publish"
	"github.com/comagnaw/regattaClock/internal/publish/spreadsheet"
	"github.com/comagnaw/regattaClock/internal/publish/webjson"
)

// Kind describes one destination kind. New is nil for a kind that is not
// built yet (Available false): it is listed, so it can be shown as coming
// soon, but cannot be published to.
type Kind struct {
	ID          string // stored in finish.json (store.PublishSettings)
	Label       string // operator-facing name
	NeedsFolder bool   // publishes a file into a folder the PFT chooses
	Available   bool
	New         func() publish.Destination
}

// Kinds is every destination kind, in display order. New kinds are added by
// appending a struct literal, as in persona.Registry.
var Kinds = []Kind{
	{
		ID:          store.DestinationSpreadsheet,
		Label:       common.DestinationSpreadsheetLabel,
		NeedsFolder: true,
		Available:   true,
		New:         func() publish.Destination { return spreadsheet.Destination{} },
	},
	{
		ID:          store.DestinationWebJSON,
		Label:       common.DestinationWebJSONLabel,
		NeedsFolder: true,
		Available:   true,
		New:         func() publish.Destination { return webjson.Destination{} },
	},
	{
		ID:    store.DestinationRegattaCentral,
		Label: common.DestinationRegattaCentralLabel,
	},
}

// ByID returns the kind with id, and false for an unknown one (say, a kind
// written by a newer build into a shared finish.json).
func ByID(id string) (Kind, bool) {
	for _, k := range Kinds {
		if k.ID == id {
			return k, true
		}
	}
	return Kind{}, false
}
