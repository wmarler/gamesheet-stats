# Original project prompt

This is the prompt that this project was built from, stored verbatim as
requested, except that the account password has been replaced with
`[REDACTED – see config.json, which is .gitignored]` so that it is not
published along with the repository.

---

Help me write a program to scrape teams.gamesheet.app for my team's statistics. My login username is will.marler@gmail.com and my password is [REDACTED – see config.json, which is .gitignored] but since there are anti-bot checks on the page, it might be better to use the stored browser data from a session I've already logged in. I have a local storage key of defaultTeamId-39NUiJGC7PfG0eKqMR8GqwTEl3M2 with a Value of bc8db8ab-f35b-4711-93d4-8eeefdb4cf03;

You can use either Python or golang for this task; create a new project in either ~/pywork or ~/gowork ; I think Golang would be better, but I don't know how hard it will be.

Here are some useful pages: https://teams.gamesheet.app/teams/bc8db8ab-f35b-4711-93d4-8eeefdb4cf03/schedule shows the games (completed and upcoming) for one of the two "Colorado 14ers 12U 2026-2027" teams I'm interested in. One of the games listed on this page is "@ WGHL Mountain Select 12U AA Girls" https://teams.gamesheet.app/teams/bc8db8ab-f35b-4711-93d4-8eeefdb4cf03/schedule/3001590 . On that page you can see that the "RMHF LHA Colorado 14ers 12U A Girls" recorded a total of 9 shots (4 in the 1st, 3 in the 2nd, and 2 in the 3rd) while the "WGHL Mountain Select 12U AA Girls" also recorded 9 shots, 2 in the 1st period, 5 in the 2nd period, and 2 in the 3rd period.

You can also see that two different goalies played for the 14ers: Sasha Speil and Emma Thompson. and that "#3" Addalyn Ma scoredat time 09:11 with an assist by "#19" Anna Skavish.

The statistics I'm interested in are: total shots taken by the 14ers; total shots faced by the 14ers; how many shots each 14ers goalie faced; (this may not be possible to discern when two goalies play a part of one period -- in that scenario calculate the number of shots faced by the % of total time played in the period for the goalie times the total number of shots in the period, rounded to the nearest whole number. It's ok if this number isn't perfect); goals and assists for each player. Please list out these statistics per game, both as human-readable output, and as a CSV file.

Please store this entire prompt in the project directory.

As a check that the program is complete, as of Friday, october 16 the 14ers 12U team has completed 12 games. We have scored 52 goals and have had 25 goals scored against us.

For the first pass at this project you do not need my approval for any code you are writing or testing. Do what you need to do to complete this task. Be sure to initiate this as a git respository, and put the sensitive data into a config file that is .gitignore'd -- I will be publishing this program to a public Github repository.
