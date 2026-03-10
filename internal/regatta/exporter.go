package regatta

import (
	"github.com/comagnaw/regattaClock/internal/exporter"
)
// loader - load excel file using dialog.
func (r *Regatta) exporter() {
	exporter.Export(*r.RegattaData)
}