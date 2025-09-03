# regattaClock

![GitHub Release](https://img.shields.io/github/v/release/comagnaw/regattaClock) ![GitHub License](https://img.shields.io/github/license/comagnaw/regattaClock) ![Go version](https://img.shields.io/github/go-mod/go-version/comagnaw/regattaClock)

**regattaClock** is an open-source project built with the goal of collecting and publishing race times for rowing organizations that hold Regattas and have limited timing resources.  At the time of this writing, **regattaClock** is in alpha development with the focus on delivering the time collection goal.  The publishing aspect is still a work in progress.  

The underlying source code is written in Golang and uses the [fyne.io](https://fyne.io/) development toolkit to build the user interface (UI).

## Overview of time collection using regattaClock

> Due to limited resources, some rowing organizations may be limited in their capability for collecting start and finish times in a cohesive system.  The **regattaClock** application is designed to be run in this environments.  What follows is a high-level overview of how **regattaClock** is meant to be run.

**regattaClock** is run at the finish-line of the race course.  **regattaClock** starts collecting time when the operator selects the `Start` button when the first boat crosses the finish-line and selects the `Lap` button each time another boat crosses the finish-line.  When the race course is clear of boats, the operator selects the `Stop` button.  The operator has collected split times for each boat.  

Once all the boats have finished the race, the Regatta Referee will provide the finish-line officials the winning time, which is the total time for the first boat to finish the race, from start to finish.  The winning time is input to **regattaClock**, which calculates all boat times based on the split times collected.  

The remaining race detail that needs to be collected is the order-of-finish (OOF), meaning which lane on the race course came in 1st, 2nd, 3rd, 4th, etc.  The operator will select the first box in the OOF column and place the lane number for the boat that came in 1st.  Once the operator hits return, the cursor will move to the next OOF box (representing 2nd place) and the operator will place the lane number for the boat that came in 2nd.  The operator will continue this operation until all places have a lane number in the OOF column.  

With all of these details collected, the results of a race can be reviewed by officials for approval and then published as an official race result time.

## Regatta Data Input

For better or worse, the only input for Regatta data is in the format of an Microsoft Excel spreadsheet (xlsx only).  Over time, this may change to a better structured input, but at the time of the initial development, the rowing organization that **regattaClock** was develped for used Excel spreadsheets as a means to organize Regatta race informaiton.

Below is a visual of the expected format of the an Excel spreadsheet, which represents all the races in the scheduled regatta.  This format is in a style for what a rowing organization may use to publish the final results of each race.  The data from this format of the spreadsheet is used to load the Regatta title, date, and informaiton for each race.  A copy of the example Excel spreadsheet can be downloaded from the [testdata](testdata) directory and used to build out your Regatta data.

![Example Input](docs/img/0-Example%20Input.png)

Below are screenshots for the various steps a operator will expereince when running **regattaClock**:

## Initial regattaClock Load

When the operator starts **regattaClock**, they will be prompted with a loading screen.  The `Load` button will open a screen with a view of the operators file system.  Ensure the operator has access to the Excel spreadsheet described in described in [Regatta Data Input](#regatta-data-input).

![Start-up Dialog](docs/img/1-Start-up%20Dialog.png)

## Input Selection

When the operator selects the load button from the previous screen, they will want to navigate to the Excel spreadsheet with Regatta data that matches the format described in [Regatta Data Input](#regatta-data-input). Once the operator has selected the proper excel spreadsheet, select `Open`.

![Input Selection](docs/img/2-Input%20Selection.png)

## Successful Load

When the Excel spreadsheet successfully loads, the operator will be presented with a window that has the Regatta title, date, and scheduled races.  This will be the primary window used to navigate collecting times for each race of the Regatta.  The `Time Race` button will be used to collect times for each race.

![Successful Load](docs/img/3-Successful%20Load.png)

## Start Timing

One the `Time Race` button is selected, a window similar to the one below will be presented to the operator.  The operator will select the `Start` button (or F2) when the first boat crosses the finish-line.  Then the operator will select the `Lap` button (or F4) as each remaining boat crosses the finish-line.  Once all boats have crossed the finish line, the operator will select the `Stop` button to stop the running clock.  Next, the operator will capture the Referee race time in the `Winning Time` box and then the OOF.

![Start Timing](docs/img/4-Start%20Timing.gif)
