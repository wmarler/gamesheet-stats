// Package stats turns a parsed scoresheet plus calendar metadata into the
// per-game statistics we care about for our own team.
package stats

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/wmarler/gamesheet-stats/internal/gamesheet"
	"github.com/wmarler/gamesheet-stats/internal/scoresheet"
)

// GameStats are our team's statistics for one completed game.
type GameStats struct {
	GameID   string
	Date     string // YYYY-MM-DD
	SeasonID int
	League   string
	Division string
	Location string
	GameNo   string

	// GameType is League, Tournament, Exhibition, or Playoff. Empty if unknown.
	GameType string
	// Ended is "" for regulation, "OT" for overtime, or "SO" for a shootout.
	Ended string
	// EmptyNetAgainst is empty-net goals scored against us. Known is false
	// when the scoresheet doesn't show whether a goal was into an empty net.
	EmptyNetAgainst int
	EmptyNetKnown   bool

	TeamName string // our team's name as printed on this scoresheet
	Opponent string
	IsHome   bool
	Result   string // W / L / T
	GoalsFor int
	GoalsAgt int

	PeriodLabels []string // e.g. "1","2","3"
	ShotsFor     []int    // shots we took, per period
	ShotsForTot  int
	ShotsAgt     []int // shots we faced, per period
	ShotsAgtTot  int

	Goalies   []GoalieGame
	Goals     []GoalEvent // our scoring plays, with names resolved
	Penalties []PenaltyEvent
	Players   []PlayerGame // every dressed skater/goalie with G/A for this game
	Against   []GoalEvent  // opponent scoring plays (numbers/names of opponent)
	Warnings  []string
}

// GoalieGame is one of our goalies' lines for one game.
type GoalieGame struct {
	Number    string
	Name      string
	Shots     []int // shots faced per period
	Total     int
	Minutes   float64
	Estimated bool // true when per-period shots were prorated from minutes played
	Dressed   bool // on the roster as G but did not appear in the shots table

	// GoalsAgainst is set only when every goal against can be given to a
	// goalie (or ruled an empty-net goal). Otherwise GoalsAgainstKnown is false
	// and the CSV cell is left blank.
	GoalsAgainst      int
	GoalsAgainstKnown bool
}

// GoalEvent is a scoring play with names resolved.
type GoalEvent struct {
	Period  string
	Time    string
	Scorer  Person
	Assists []Person
	// Situation is EV, PP, SH (shootout), EN, or PS. Empty when it cannot be
	// determined, including a short-handed goal (that code is not in the
	// requested set; SH means shootout).
	Situation string
}

// PenaltyEvent is one of our penalties.
type PenaltyEvent struct {
	Period  string
	Player  Person
	Type    string // Tripping, Hooking, ..., Other. Empty if the code is missing.
	Minutes string // "2", "4", "5", or "10". Empty if it cannot be classified.
}

// Person is a jersey number plus name.
type Person struct {
	Number string
	Name   string
}

func (p Person) String() string {
	if p.Name == "" {
		return "#" + p.Number
	}
	return fmt.Sprintf("#%s %s", p.Number, p.Name)
}

// PlayerGame is a player's line for one game.
type PlayerGame struct {
	Number   string
	Name     string
	Position string // "G" or ""
	Goals    int
	Assists  int
}

func (p PlayerGame) Points() int { return p.Goals + p.Assists }

// Build computes our stats for one game from the calendar entry and the
// parsed scoresheet.
func Build(g gamesheet.Game, sheet *scoresheet.Scoresheet) (*GameStats, error) {
	us, them := &sheet.Visitor, &sheet.Home
	if g.EventData.IsHome {
		us, them = &sheet.Home, &sheet.Visitor
	}
	gs := &GameStats{
		GameID:   g.ID,
		Date:     g.EventDate,
		SeasonID: g.EventData.SeasonID,
		League:   sheet.League,
		Division: sheet.Division,
		Location: firstNonEmpty(g.EventLocation, sheet.Location),
		GameNo:   firstNonEmpty(sheet.GameNumber, g.EventData.GameNumber),
		GameType: sheet.GameType,
		TeamName: firstNonEmpty(us.Name, g.MyTeamName()),
		Opponent: firstNonEmpty(them.Name, g.Opponent()),
		IsHome:   g.EventData.IsHome,
		GoalsFor: us.Goals,
		GoalsAgt: them.Goals,
	}

	// Sanity-check the sheet against the calendar.
	if g.EventData.HomeGoals != nil && g.EventData.VisitorGoals != nil {
		gf, ga := *g.EventData.VisitorGoals, *g.EventData.HomeGoals
		if g.EventData.IsHome {
			gf, ga = ga, gf
		}
		if gf != gs.GoalsFor || ga != gs.GoalsAgt {
			gs.warn("score on scoresheet (%d-%d) differs from calendar (%d-%d); using calendar", gs.GoalsFor, gs.GoalsAgt, gf, ga)
			gs.GoalsFor, gs.GoalsAgt = gf, ga
		}
	}
	if n := len(us.Scores); n != gs.GoalsFor {
		gs.warn("%d scoring plays listed for us but %d goals recorded", n, gs.GoalsFor)
	}
	switch {
	case gs.GoalsFor > gs.GoalsAgt:
		gs.Result = "W"
	case gs.GoalsFor < gs.GoalsAgt:
		gs.Result = "L"
	default:
		gs.Result = "T"
	}

	// Shots. The SHOTS table on a team's side records shots faced by that
	// team's goalies, i.e. shots taken by the opponent.
	gs.PeriodLabels = firstNonEmptySlice(us.ShotColumns, them.ShotColumns)
	gs.ShotsFor, gs.ShotsForTot = them.ShotsFaced, them.ShotsFacedTotal
	gs.ShotsAgt, gs.ShotsAgtTot = us.ShotsFaced, us.ShotsFacedTotal
	if gs.ShotsFor == nil {
		gs.warn("no shots table found on opponent side (our shots unknown)")
	}
	if gs.ShotsAgt == nil {
		gs.warn("no shots table found on our side (shots against unknown)")
	}

	gs.Goalies = buildGoalies(us, gs)

	// Scoring.
	resolve := func(t *scoresheet.TeamSheet, num string) Person {
		p := Person{Number: num}
		if pl := t.FindPlayer(num); pl != nil {
			p.Name = PrettyName(pl.Name)
		}
		return p
	}
	for _, sc := range us.Scores {
		ev := GoalEvent{Period: sc.Period, Time: sc.Time, Scorer: resolve(us, sc.Scorer)}
		for _, a := range sc.Assists {
			ev.Assists = append(ev.Assists, resolve(us, a))
		}
		gs.Goals = append(gs.Goals, ev)
	}
	for _, sc := range them.Scores {
		ev := GoalEvent{Period: sc.Period, Time: sc.Time, Scorer: resolve(them, sc.Scorer)}
		for _, a := range sc.Assists {
			ev.Assists = append(ev.Assists, resolve(them, a))
		}
		gs.Against = append(gs.Against, ev)
	}

	// Players: everyone dressed, with their G/A. (Index map rather than
	// pointers, since appends may reallocate the slice.)
	idx := map[string]int{}
	player := func(num string) *PlayerGame {
		if i, ok := idx[num]; ok {
			return &gs.Players[i]
		}
		return nil
	}
	addPlayer := func(p PlayerGame) *PlayerGame {
		gs.Players = append(gs.Players, p)
		idx[p.Number] = len(gs.Players) - 1
		return &gs.Players[len(gs.Players)-1]
	}
	for _, pl := range us.Roster {
		if player(pl.Number) != nil {
			gs.warn("jersey #%s appears more than once on the roster", pl.Number)
			continue
		}
		addPlayer(PlayerGame{Number: pl.Number, Name: PrettyName(pl.Name), Position: pl.Position})
	}
	for _, sc := range us.Scores {
		if sc.Scorer != "" {
			p := player(sc.Scorer)
			if p == nil {
				gs.warn("goal credited to #%s who is not on the roster", sc.Scorer)
				p = addPlayer(PlayerGame{Number: sc.Scorer})
			}
			p.Goals++
		}
		for _, a := range sc.Assists {
			p := player(a)
			if p == nil {
				gs.warn("assist credited to #%s who is not on the roster", a)
				p = addPlayer(PlayerGame{Number: a})
			}
			p.Assists++
		}
	}
	finishGame(gs, us, them)
	sort.SliceStable(gs.Players, func(i, j int) bool {
		pi, pj := gs.Players[i], gs.Players[j]
		if pi.Points() != pj.Points() {
			return pi.Points() > pj.Points()
		}
		if pi.Goals != pj.Goals {
			return pi.Goals > pj.Goals
		}
		return pi.Name < pj.Name
	})
	return gs, nil
}

// buildGoalies produces our goalie lines. When the scoresheet gives
// per-goalie, per-period shot counts (the normal case) they are used
// directly. If a goalie row only has minutes played, the shots not already
// attributed to other goalies are prorated by share of minutes played and
// rounded to the nearest whole number.
func buildGoalies(us *scoresheet.TeamSheet, gs *GameStats) []GoalieGame {
	nPer := len(gs.PeriodLabels)
	var out []GoalieGame
	nameOf := func(num string) string {
		if pl := us.FindPlayer(num); pl != nil {
			return PrettyName(pl.Name)
		}
		return ""
	}

	known := make([]int, nPer)
	knownTotal := 0
	var unknownIdx []int
	for _, gl := range us.Goalies {
		gg := GoalieGame{Number: gl.Number, Name: nameOf(gl.Number), Minutes: gl.Minutes, Total: gl.Total}
		if gl.HasShots {
			gg.Shots = append([]int(nil), gl.Shots...)
			for i, s := range gl.Shots {
				if i < nPer {
					known[i] += s
				}
			}
			knownTotal += gl.Total
		} else {
			unknownIdx = append(unknownIdx, len(out))
		}
		out = append(out, gg)
	}

	// No per-goalie rows at all: attribute everything to the lone dressed
	// goalie if there is exactly one, otherwise leave it unattributed.
	if len(out) == 0 && gs.ShotsAgt != nil {
		goalies := us.GoalieRoster()
		if len(goalies) == 1 {
			out = append(out, GoalieGame{
				Number: goalies[0].Number, Name: PrettyName(goalies[0].Name),
				Shots: append([]int(nil), gs.ShotsAgt...), Total: gs.ShotsAgtTot, Minutes: us.MinutesTotal,
			})
		} else if len(goalies) > 1 {
			gs.warn("shots table has no per-goalie rows and %d goalies dressed; shots faced per goalie unknown", len(goalies))
			for _, g := range goalies {
				out = append(out, GoalieGame{Number: g.Number, Name: PrettyName(g.Name), Dressed: true})
			}
		}
		return out
	}

	// Prorate for goalie rows that have minutes but no shot counts.
	if len(unknownIdx) > 0 && gs.ShotsAgt != nil {
		var minutes float64
		for _, i := range unknownIdx {
			minutes += out[i].Minutes
		}
		for _, i := range unknownIdx {
			g := &out[i]
			share := 0.0
			if minutes > 0 {
				share = g.Minutes / minutes
			} else {
				share = 1 / float64(len(unknownIdx))
			}
			g.Shots = make([]int, nPer)
			g.Total = 0
			for p := 0; p < nPer && p < len(gs.ShotsAgt); p++ {
				remaining := gs.ShotsAgt[p] - known[p]
				if remaining < 0 {
					remaining = 0
				}
				g.Shots[p] = int(math.Round(float64(remaining) * share))
				g.Total += g.Shots[p]
			}
			g.Estimated = true
		}
		gs.warn("shots faced for %d goalie(s) estimated from minutes played", len(unknownIdx))
	} else if gs.ShotsAgt != nil && knownTotal != gs.ShotsAgtTot {
		gs.warn("per-goalie shots (%d) do not add up to team total (%d)", knownTotal, gs.ShotsAgtTot)
	}

	// Note dressed goalies who never appeared in the shots table.
	for _, g := range us.GoalieRoster() {
		found := false
		for _, o := range out {
			if o.Number == g.Number {
				found = true
				break
			}
		}
		if !found {
			out = append(out, GoalieGame{Number: g.Number, Name: PrettyName(g.Name), Dressed: true})
		}
	}
	return out
}

func (gs *GameStats) warn(format string, args ...any) {
	gs.Warnings = append(gs.Warnings, fmt.Sprintf(format, args...))
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstNonEmptySlice(vals ...[]string) []string {
	for _, v := range vals {
		if len(v) > 0 {
			return v
		}
	}
	return nil
}

// PrettyName converts an upper-case scoresheet name such as "GREGORY ROWE II"
// or "ADDALYN MA" into mixed case: "Gregory Rowe II", "Addalyn Ma".
func PrettyName(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		words[i] = capitalizeWord(w)
	}
	return strings.Join(words, " ")
}

func capitalizeWord(w string) string {
	switch w {
	case "ii", "iii", "iv", "jr.", "sr.", "jr", "sr":
		if strings.HasPrefix(w, "i") {
			return strings.ToUpper(w)
		}
		return strings.ToUpper(w[:1]) + w[1:]
	}
	// Handle hyphenated and apostrophe'd names piecewise.
	for _, sep := range []string{"-", "'"} {
		if strings.Contains(w, sep) {
			parts := strings.Split(w, sep)
			for i, p := range parts {
				parts[i] = capitalizeWord(p)
			}
			return strings.Join(parts, sep)
		}
	}
	if strings.HasPrefix(w, "mc") && len(w) > 2 {
		return "Mc" + capitalizeWord(w[2:])
	}
	if w == "" {
		return w
	}
	return strings.ToUpper(w[:1]) + w[1:]
}
