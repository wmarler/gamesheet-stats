# gamesheet-stats

Pulls a youth hockey team's completed games from
[GameSheet Teams](https://teams.gamesheet.app) and reports, per game and for
the season:

- shots taken and shots faced (by period)
- shots faced by each goalie (by period) and minutes played
- goals and assists for every player
- every scoring play (ours and the opponent's)

Output is printed as human-readable text and written to CSV files.

## How it works

`teams.gamesheet.app` is a React app; the box score it shows is an embedded
widget from `gamesheetstats.com`, which sits behind a Cloudflare JavaScript
challenge and cannot be fetched by a plain HTTP client. The Teams API gateway
(`api.teams.gamesheet.app`) is **not** challenged, so this tool talks to it
directly, the same way the web app does:

1. Sign in with Firebase email/password (Google Identity Toolkit REST API).
2. Exchange the Firebase ID token for GameSheet gateway tokens
   (`GET /api/auth/tokens`), cached in `cache/tokens.json` and refreshed via
   `POST /api/auth/refresh` so that sign-in is rarely repeated.
3. `GET /api/calendar?teamId=…&month=all&eventType=game` lists every game
   with score, status and season.
4. For each completed ("final") game, download the official scoresheet PDF
   (`GET /api/scoresheets/games/{id}/download`) and parse it.

The scoresheet PDF contains everything we need: rosters, scoring plays,
penalties, the checked game-type box, and a SHOTS table with shots per period
per goalie plus minutes played. The PDF is parsed with a pure-Go library
using text coordinates (no poppler/pdftotext needed). PDFs are cached in
`cache/pdfs/`. When the game-type checkbox cannot be read, the program falls
back to the schedule API's `game_type`.

### Goalie shot attribution

The scoresheet normally lists per-goalie, per-period shots, so those exact
numbers are used. If a goalie row only has minutes played (no shot cells), the
shots not already attributed to other goalies are prorated by share of minutes
played and rounded to the nearest whole number; such rows are marked
`estimated` in the output.

## Setup

Requires Go 1.22+.

```sh
cp config.example.json config.json
# edit config.json: email, password, and team_ids
go build -o gamesheet-stats .
./gamesheet-stats -list-teams     # find your team IDs
./gamesheet-stats                 # run the report
```

`config.json` holds your credentials and is listed in `.gitignore`; never
commit it. `team_ids` are the UUIDs that appear in Teams URLs
(`https://teams.gamesheet.app/teams/<team_id>/schedule`). If `team_ids` is
empty, every non-archived team visible to the account is used.

Optional config keys: `api_base` and `firebase_api_key` override the
production defaults compiled into the binary.

## Usage

```
./gamesheet-stats [flags]

  -config string   path to config file (default "config.json")
  -out string      directory for CSV output (default "output")
  -cache string    directory for cached tokens and PDFs (default "cache")
  -team string     comma-separated team IDs (overrides config)
  -refresh         re-download scoresheet PDFs even if cached
  -list-teams      list teams visible to this account and exit
  -no-csv          do not write CSV files
  -quiet           suppress progress messages on stderr
```

Progress goes to stderr; the report goes to stdout, so
`./gamesheet-stats > report.txt` captures just the report.

### CSV files

Three files are written. `Game #` is the game's place in `game_log.csv`,
starting at 1 for the earliest game. `goal_log.csv` and `penalty_log.csv` use
that same number. A cell is left blank when the scoresheet does not say.

`game_log.csv` — one row per game:

| Column | Values |
|---|---|
| Game #, Date, Opponent | |
| Home/Away | `Home` or `Away` |
| Game Type | `League`, `Tournament`, `Exhibition`, or `Playoff` |
| Goals-For, Shots-For | |
| Goalie 1 / Goalie 2 | name, minutes, shots faced, goals against. Only goalies who played. Goals against is blank when two goalies split the period the goal was scored in |
| Empty-net Goals Against | blank when it cannot be told apart from a goal on a goalie |
| How the game ended | blank for regulation, `OT`, or `SO` |

`goal_log.csv` — one row per goal we scored: Game #, Period, Scorer, Assist 1,
Assist 2, Situation. Situation is `EV` even strength, `PP` power play, `SH`
shootout, `EN` empty net, or `PS` penalty shot. A short-handed goal is left
blank, because `SH` means shootout in this file. Power play is decided from
penalty start/return times on the scoresheet (the clock counts down).

`penalty_log.csv` — one row per penalty we took: Game #, Period, Player,
Penalty Type, Minutes. Penalty Type is one of Tripping, Hooking, Holding,
Slashing, Interference, Roughing, Cross-checking, High-sticking, Boarding,
Body Checking, Delay of Game, Too Many Players, Unsportsmanlike, Misconduct,
or Other. Minutes is `2`, `4`, `5`, or `10`. Running-time sheets print a minor
as 1 or 1.5 minutes; those are recorded as `2` from the penalty code.

Note: a player's jersey number can differ between leagues; the season summary
on stdout matches players by name.

## Development

```sh
go test ./...
```

The parser tests use synthetic text runs laid out like a real scoresheet so
that no real scoresheets (with children's names) need to be committed.

## Project layout

```
main.go                      CLI
internal/gamesheet/          config, Firebase/gateway auth, Teams API client
internal/scoresheet/         scoresheet PDF text extraction and layout parsing
internal/stats/              per-game stats and season summary
internal/report/             text and CSV output
PROMPT.md                    the original prompt this project was built from
```
