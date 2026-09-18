# Patches before release

## Bug 1 - Cosmetic

- After RD challenge, the input for xlsx and directory is lacking contrast in the light theme.  The text needs to be darker.  We probably need to check the theme across all the windows to make sure consistency.

## Bug 2 - Unexpected outcome

- As RD, after directory selection, the regattaData directory is immediately created.  This should not happen until the user selects Start Regatta button.  The user may have selected the incorrect directory and wants to change the directory selection.  This would create an empty directory.

## Bug 3 - Not Responding

- The app running on one windows machine went non-responsive between the persona challenge and the window which let you select the directory for the regattaData.  This happened each time I changed persona.  I think it may have to do with not finding a preferences directory to find a previous directory.  I say this because the directory select window goes to a user home directory and not the directory I was previously working in as previous persona.  I am looking for suggestions to troubleshoot this issue.

## Feature 1 - Cosmetic

- In version window, the Source url should not include the commit.  This is a bit confusing when clicked on.  Just link to the base repo url.

## Feature 2 - Cosmetic

- FT Wall Clock Window bottom status banner could use some contrast.  I would think the blue background and white text that we use for clock and results banner.

## Feature 3 - Cosmetic

- race tree "Scheduled Races" column would be nice to include lane count in each race which includes boats (" - 4 Lanes")

## Feature 4 - Cosmetic

- race tree for RD should model itself Award persona.  The RD should have the ability to see the results just like Award persona.  The difference between RD and Award is that RD has ability to write (via watching xlsx and selecting directory) and Award is read only persona.

## Feature 5 - Cosmetic

- when PFT hits stop on wall clock, the status update that other personas race tree should reflect "under review", this is after "timing in progress" status.

## Feature 6 - Cosmetic

- race tree sub-title banner with regatta meta-data, alignment of text should be adjusted so that the : lines up between the two rows.  This may require a 2x8 matrix.  First column with Key would be right adjusted, the second column would be left adjusted, repeat that pattern for the remainder of the columns.