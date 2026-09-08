# Example regatta workbooks

Two sample Excel workbooks, one for each format regattaClock accepts: a plain
`.xlsx` and a macro-enabled `.xlsm`.

On import, regattaClock reads the worksheet named **`Results`** — or the first
worksheet if the file has no sheet by that name — and derives the regatta title
and date and each race's lane assignments from its layout. Other worksheets,
macros, and formulas are ignored.

## Example Regatta Input Table.xlsx

The minimal input: a single worksheet in the `Results` layout, holding one
regatta's title, date, and per-race lane assignments. regattaClock imports it
as-is.

## Example Heat Sheets and Results With Macros.xlsm

A fuller, self-contained workbook for running a whole regatta.

The `Regatta Attributes` worksheet is the main input for the regatta's metadata.
Its buttons are wired to macros and do nothing when macros are disabled; each one
exports another worksheet to a standalone, macro-free workbook — useful for
sending just the `Heat Sheet`, `Results`, or `Referee Heat Sheet` without the
formulas and macros.

The `Heat Sheet` worksheet is the source for the races and their lane
assignments; the other worksheets pull their data from it through formulas. The
`Results` worksheet is where timing and order of finish (OOF) are entered, and it
is the sheet regattaClock imports.

Because the workbook leans heavily on formulas to keep every sheet consistent,
some cell ranges are protected. If you try to edit one, Excel warns:

> The cell or chart you're trying to change is on a protected sheet. To make a
> change, unprotect the sheet. You might be requested to enter a password.

You can unprotect a sheet, but do so carefully — the protection keeps the
formulas from being accidentally changed or deleted.
