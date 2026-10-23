package stats

import (
	"strconv"
	"strings"

	"github.com/wmarler/gamesheet-stats/internal/scoresheet"
)

// finishGame fills in everything that depends on the whole scoresheet:
// goal situation, penalty log, how the game ended, empty-net goals against,
// and goals allowed by each goalie.
func finishGame(gs *GameStats, us, them *scoresheet.TeamSheet) {
	for i := range gs.Goals {
		gs.Goals[i].Situation = goalSituation(gs.Goals[i], us.Penalties, them.Penalties, them)
	}
	for i := range gs.Against {
		gs.Against[i].Situation = goalSituation(gs.Against[i], them.Penalties, us.Penalties, us)
	}
	for _, p := range us.Penalties {
		ev := PenaltyEvent{Period: p.Period, Type: penaltyType(p.Code), Minutes: penaltyMinutes(p)}
		if p.Number != "" {
			ev.Player = person(us, p.Number)
		}
		gs.Penalties = append(gs.Penalties, ev)
	}
	gs.Ended = gameEnded(gs)
	attributeGoalsAgainst(gs, us)
}

// goalSituation classifies one of our goals. ourPens are the penalties of the
// team that scored; theirPens are the defending team's penalties. net is the
// defending team's sheet (its shots-faced table is shots on its goalies).
func goalSituation(g GoalEvent, ourPens, theirPens []scoresheet.Penalty, net *scoresheet.TeamSheet) string {
	if isShootout(g.Period) {
		return "SH"
	}
	if shots, ok := periodShots(net, g.Period); ok && shots == 0 {
		return "EN"
	}
	ourN, ourOK := playersServing(ourPens, g.Period, g.Time)
	theirN, theirOK := playersServing(theirPens, g.Period, g.Time)
	if !ourOK || !theirOK {
		return ""
	}
	switch {
	case theirN > ourN:
		return "PP"
	case ourN > theirN:
		// Short-handed. The requested codes use SH for a shootout, so leave
		// this blank rather than mislabel it.
		return ""
	default:
		return "EV"
	}
}

// playersServing counts penalties being served at the goal's clock time.
// The scoresheet clock counts down, and a penalty is active while the time
// remaining is below its start and at or above the time the player returned.
// The second result is false when a penalty in that period might have been
// active but its start or return time is missing.
func playersServing(pens []scoresheet.Penalty, period, goalTime string) (int, bool) {
	goal, ok := clockSeconds(goalTime)
	if !ok {
		if len(pens) == 0 {
			return 0, true
		}
		return 0, false
	}
	n := 0
	for _, p := range pens {
		if p.Period != period {
			continue
		}
		start, okS := clockSeconds(p.Start)
		end, okE := clockSeconds(p.On)
		if !okS || !okE {
			// A penalty that had not started yet cannot cover this goal.
			if okS && goal >= start {
				continue
			}
			return 0, false
		}
		if goal < start && goal >= end {
			n++
		}
	}
	return n, true
}

func clockSeconds(s string) (int, bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, false
	}
	m, errM := strconv.Atoi(parts[0])
	sec, errS := strconv.Atoi(parts[1])
	if errM != nil || errS != nil || sec < 0 || sec >= 60 || m < 0 {
		return 0, false
	}
	return m*60 + sec, true
}

// periodShots returns the shots-faced total for one period on a team's sheet.
func periodShots(t *scoresheet.TeamSheet, period string) (int, bool) {
	if t == nil || t.ShotsFaced == nil {
		return 0, false
	}
	for i, label := range t.ShotColumns {
		if label == period && i < len(t.ShotsFaced) {
			return t.ShotsFaced[i], true
		}
	}
	return 0, false
}

func gameEnded(gs *GameStats) string {
	so, ot := false, false
	note := func(period string) {
		switch {
		case isShootout(period):
			so = true
		case isOvertime(period):
			ot = true
		}
	}
	for _, g := range gs.Goals {
		note(g.Period)
	}
	for _, g := range gs.Against {
		note(g.Period)
	}
	for _, p := range gs.PeriodLabels {
		note(p)
	}
	if so {
		return "SO"
	}
	if ot {
		return "OT"
	}
	// A shootout winner is often not written in the scoring list. When the
	// listed regulation goals are tied and the final score differs by exactly
	// one, that missing goal is the shootout.
	regFor, regAgt := 0, 0
	for _, g := range gs.Goals {
		if isRegulation(g.Period) {
			regFor++
		}
	}
	for _, g := range gs.Against {
		if isRegulation(g.Period) {
			regAgt++
		}
	}
	listed := len(gs.Goals) + len(gs.Against)
	final := gs.GoalsFor + gs.GoalsAgt
	if regFor == regAgt && listed+1 == final && absInt(gs.GoalsFor-gs.GoalsAgt) == 1 {
		return "SO"
	}
	return ""
}

func isRegulation(period string) bool {
	return period == "1" || period == "2" || period == "3"
}

func isOvertime(period string) bool {
	p := strings.ToUpper(period)
	return p == "OT" || strings.HasPrefix(p, "OT") || p == "4"
}

func isShootout(period string) bool {
	p := strings.ToUpper(period)
	return p == "SO" || strings.HasPrefix(p, "SO")
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// attributeGoalsAgainst gives each goal against to a goalie when the shots
// table shows only one of our goalies faced shots in that period. A goal in a
// period with no shots on goal is an empty-net goal. Anything else (two
// goalies splitting a period, or no shots table) leaves the unknown cells blank.
func attributeGoalsAgainst(gs *GameStats, us *scoresheet.TeamSheet) {
	// The calendar score can include a goal the scoring list does not (the
	// overtime winner on 2026-10-18 is one). Without that goal we cannot say
	// who allowed it or whether it was an empty net.
	if len(gs.Against) != gs.GoalsAgt {
		return
	}
	played := []int{}
	for i := range gs.Goalies {
		if !gs.Goalies[i].Dressed {
			played = append(played, i)
		}
	}
	assigned := make([]int, len(gs.Goalies))
	uncertain := make([]bool, len(gs.Goalies))
	en, enKnown := 0, us.ShotsFaced != nil
	for _, g := range gs.Against {
		if g.Situation == "EN" {
			en++
			continue
		}
		shots, ok := periodShots(us, g.Period)
		if !ok {
			enKnown = false
			for _, i := range played {
				uncertain[i] = true
			}
			continue
		}
		if shots == 0 {
			en++
			continue
		}
		cands := candidates(gs, played, g.Period)
		if len(cands) == 1 {
			assigned[cands[0]]++
			continue
		}
		if len(cands) == 0 {
			// Shots were recorded but no goalie row owns them.
			enKnown = false
		}
		for _, i := range cands {
			uncertain[i] = true
		}
	}
	gs.EmptyNetAgainst = en
	gs.EmptyNetKnown = enKnown
	for _, i := range played {
		// A prorated shots line is only a problem when another goalie also
		// played; a lone goalie faced every non-empty-net goal.
		if uncertain[i] || (gs.Goalies[i].Estimated && len(played) > 1) {
			continue
		}
		gs.Goalies[i].GoalsAgainst = assigned[i]
		gs.Goalies[i].GoalsAgainstKnown = true
	}
}

// candidates are the goalies who could have been in net for a period: a
// goalie with per-period shots only if they faced at least one, otherwise
// (no per-period breakdown) every goalie who played.
func candidates(gs *GameStats, played []int, period string) []int {
	var certain, unknown []int
	for _, i := range played {
		g := gs.Goalies[i]
		if len(g.Shots) == 0 || g.Estimated {
			unknown = append(unknown, i)
			continue
		}
		if n, ok := goaliePeriodShots(gs, g, period); ok && n > 0 {
			certain = append(certain, i)
		}
	}
	if len(unknown) > 0 {
		return append(certain, unknown...)
	}
	return certain
}

func goaliePeriodShots(gs *GameStats, g GoalieGame, period string) (int, bool) {
	for i, label := range gs.PeriodLabels {
		if label == period && i < len(g.Shots) {
			return g.Shots[i], true
		}
	}
	return 0, false
}

func person(t *scoresheet.TeamSheet, number string) Person {
	p := Person{Number: number}
	if pl := t.FindPlayer(number); pl != nil {
		p.Name = PrettyName(pl.Name)
	}
	return p
}

// penaltyType maps a GameSheet infraction code (for example "TR-MIN") onto
// the requested label. A code we don't recognise is "Other"; a missing code
// is "".
func penaltyType(code string) string {
	if code == "" {
		return ""
	}
	base := strings.ToUpper(code)
	if i := strings.Index(base, "-"); i >= 0 {
		base = base[:i]
	}
	switch base {
	case "TR":
		return "Tripping"
	case "HK", "HKG":
		return "Hooking"
	case "HO", "HLD":
		return "Holding"
	case "SL":
		return "Slashing"
	case "INT", "INTB", "INTG":
		return "Interference"
	case "RO", "RGH", "RAW":
		return "Roughing"
	case "CC":
		return "Cross-checking"
	case "HS":
		return "High-sticking"
	case "BDG":
		return "Boarding"
	case "BC":
		return "Body checking"
	case "DG", "DOG":
		return "Delay of Game"
	case "TMP", "TMM":
		return "Too many players"
	case "USP", "USC", "UNS", "DAB":
		return "Unsportsmanlike"
	case "MSC", "GM", "GRM", "GE":
		return "Misconduct"
	default:
		return "Other"
	}
}

// penaltyMinutes reports 2, 4, 5, or 10. The printed duration is used when it
// is already one of those. Running-time scoresheets print 1 or 1.5 for a
// minor, so the code's class (MIN / MAJ / GM) decides instead. "" if neither
// tells us.
func penaltyMinutes(p scoresheet.Penalty) string {
	if p.HasMinutes {
		switch p.Minutes {
		case 2, 4, 5, 10:
			return strconv.Itoa(int(p.Minutes))
		}
	}
	class := ""
	if i := strings.LastIndex(p.Code, "-"); i >= 0 {
		class = strings.ToUpper(p.Code[i+1:])
	}
	switch class {
	case "MIN":
		return "2"
	case "MAJ", "MP":
		return "5"
	case "GM", "MSC", "GRM", "GE":
		return "10"
	default:
		return ""
	}
}
