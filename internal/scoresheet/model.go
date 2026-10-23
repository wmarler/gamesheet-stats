// Package scoresheet parses the official GameSheet scoresheet PDF (the one
// downloadable from the Teams app) into structured data.
//
// The PDF is a single landscape page laid out as two mirrored halves: HOME on
// the left, VISITOR on the right. Each half contains the roster, penalties,
// scoring plays, and a "SHOTS" table listing shots faced per period by each
// goalie plus a TOTAL row. Because the shots table is on the goaltending
// team's side, HOME's shots table == shots taken by VISITOR, and vice versa.
package scoresheet

// Scoresheet is the parsed content of one scoresheet PDF.
type Scoresheet struct {
	GameNumber string
	Date       string // as printed, e.g. "Sep 19, 2026"
	Division   string
	League     string
	Location   string
	Status     string // e.g. "FINAL"

	// GameType is the checked box in the scoresheet "GAME TYPE" panel:
	// "League", "Tournament", "Exhibition", or "Playoff". Empty if no single
	// box is checked.
	GameType string

	// Periods as printed in the "Length of Game" box, e.g. {"1st",12},{"2nd",12},{"3rd",12}.
	Periods []Period

	Home    TeamSheet
	Visitor TeamSheet
}

// Period is a regulation/overtime period and its scheduled length in minutes.
type Period struct {
	Name    string
	Minutes float64
}

// TeamSheet is one half (HOME or VISITOR) of the scoresheet.
type TeamSheet struct {
	Name      string
	Goals     int // "Total Goals" box
	Roster    []Player
	Scores    []Goal
	Penalties []Penalty

	// ShotColumns are the period column labels of the SHOTS table, e.g. "1","2","3" (and "OT" if present).
	ShotColumns []string
	// Goalies are the per-goalie rows of the SHOTS table (shots faced by this team's goalies).
	Goalies []GoalieLine
	// ShotsFaced is the TOTAL row of the SHOTS table: shots this team's goalies faced per column.
	ShotsFaced []int
	// ShotsFacedTotal is the TOTAL cell of the TOTAL row.
	ShotsFacedTotal int
	// MinutesTotal is the minutes-played cell of the TOTAL row.
	MinutesTotal float64
}

// Player is a roster line.
type Player struct {
	Number   string
	Name     string   // as printed (upper case)
	Position string   // "G" for goalies, otherwise ""
	Flags    []string // other markers printed next to the player: C, A, AP, (INJ), ...
}

// Goal is one scoring play.
type Goal struct {
	Period  string
	Time    string
	Scorer  string   // jersey number
	Assists []string // jersey numbers (0-2)
}

// Penalty is one row of the penalties table.
type Penalty struct {
	Period     string
	Number     string  // jersey; empty for a bench penalty with no player
	Minutes    float64 // duration as printed (running-time games use 1 or 1.5 for a minor)
	HasMinutes bool
	Code       string // e.g. "TR-MIN"
	Start      string // clock time remaining when the penalty started, e.g. "10:32"
	On         string // clock time remaining when the player returned
}

// GoalieLine is one row of the SHOTS table.
type GoalieLine struct {
	Number   string
	Shots    []int // per ShotColumns; nil if the row had no per-period numbers
	HasShots bool
	Total    int
	Minutes  float64
}

// FindPlayer returns the roster entry with the given jersey number.
func (t *TeamSheet) FindPlayer(number string) *Player {
	for i := range t.Roster {
		if t.Roster[i].Number == number {
			return &t.Roster[i]
		}
	}
	return nil
}

// GoalieRoster returns the roster entries marked as goalies.
func (t *TeamSheet) GoalieRoster() []Player {
	var out []Player
	for _, p := range t.Roster {
		if p.Position == "G" {
			out = append(out, p)
		}
	}
	return out
}
