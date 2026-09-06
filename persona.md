# Persona

This file includes goals for adding personas to regattaClock

- RegattaClock needs to work with several personas
- Each persona has specific roles and restrictions on what they can do
- The personas must be able to read and write data in a shared directory location.
  - The current list of personas should be organized in source code such that future personas can easily be added later.
- RegattaClock should have a go routine that is tracking changes in the shared directory location.
  - If a change is detected, the data should be loaded.
  - As of right now, the regattaData directory is the primary location for that data.
  - A directory structure and data-model for the data related to each persona needs to be defined.
  - The implementation should be mindful of any issues related to multiple people reading/writing data in common files.
  - The data format saved to disk should be as performant as possible for reading into a go routine and writing to disk.
- The personas are as follows: 
  - Regatta Director (RD) - A user that initilizes by loading regatta data from excel and establishes the regattaData save location.
  - Start Timer (ST) - A person that is responsible for collecting a start time for each race.
  - Finish Timer (FT) - A perosn that is responsible for collecting lap, final time, and OOF for each race.
- There will be two teams for a pairing of ST and FT.
  - One team is primary and the other team is secondary.
  - The regattaData directory should account for primary ST and FT and secondary ST and FT.
  - The data saved and read from the regattaData directory should be indexed based on the personas team (primary or secondary)
- On loading of the RegattaClock app, the user should be asked what team and persona they are acting as.
  - Primary Start Timer, Secondary Start Timer, Primary Finish Timer, Secondary Finish Timer
  - There should be a simple challenge that is unique to each persona where they must fill in.
    - For right now, this can be based on a config.json file in source code or based on constant values.
    - The text should not be anything complicated but a simple textual queue (e.g. rc-pst, rc-sst, etc)
    - If the user cannot meet this challenge, they should be presented with an error and return to the question of what persona the want.
  - Upon successful persona load, the next question should be what directory to load the regattaData from.
    - They should only be able to select a directory that matches regattaData before they can click Open.
    - Upon selection, they should be presented with a confirmation of the regatta title, date, and subtitle which they must confirm is the regatta they want.
      - If it is not, then they should be presnted with the directory selection again.
      - If it is, then they should be presented with the race tree which is loaded based on their persona (ST or FT)
- The Regatta Director:
  - Should follow the current entry point for start-up of Regatta Clock.
  - However, the whole start-up and of regettaClock likely should change and not load the last preferences regattaData from data.json
    - The preferences likely does not need to retain RegattaDir and should not be used to load the last.
  - Responsible for loading the excel sheeet and saving to regattaData directory, which is currently saved a data.json.
    - However, the RD may need to pull changes from an excel sheet and update the data.json.
    - Therefore, the RD should be the only one saving to the current data.json.
    - Potentially, move data.json to a sub-directory related to RD persona under regattaData directory. 
- The Start Timer (ST):
  - should load from RD data.json to load race tree.
  - should have a button only visible to them that says "Start Time" in the race tree.
  - should not have access to the "Time Race" button that is currently in the race tree (should be hidden).
  - when "Start Time" is clicked, it should collect the HH:MM:SS.ms at the point of being clicked.
  - the time collected should reflect in the row in the race tree, next to the "Start Time" button.
  - the collected times should be saved to the regattaData directory as readable structured data for other personas.
    - the start time should be indexed in such a way to be easily found, not clear if each race should have a separate file or if the whole regatta should have one results file.
  - the collected time should be able to be cleared (in the chance the race had to restart) and be confirmed by the persona before clearing.
  - when the time is cleared, it should update the saved data in regattaData directory.
- The Finish Timers (FT):
  - should load from RD data.json to load race tree.
  - should see the ST collected time in the race tree for each race.
  - should not see the "Start Time" button in the race tree.
  - should see the "Time Race" button in the race tree.
  - As ST collect time, the FT race tree should update with the collected times (via the go routine).
  - when the FT click on "Time Race" they will collect the Lap times like what happens currently.
    - the only difference now will be that when the FT click the Start button, the Winning Time will now be able to be calculated from the difference in time between the ST collected time and FT's start button click.
  - when the FT selects Referee Approval, all the results data should be saved to regattaData directory
   - the results should be indexed in such a way to be easily found, not clear if each race should have a separate results file or if the whole regatta should have one results file.
  - the save button will perform the same data save as the Refereee Approval.
  - if the FT closes the clock window and reopens the same race from the race tree, the regattaData directory should be checked for and populate the clock window with that data.


 

