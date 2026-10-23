package scoresheet

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Column boundaries within one half of the sheet, in points, after the right
// half has been shifted left so both halves share the HOME half's coordinates.
// These come from the fixed scoresheet template (see testdata notes in
// parse_test.go); the exact text positions inside each band vary a little
// because cells are centred, so we classify by band rather than exact X.
const (
	rosterNumMaxX  = 55  // jersey number column
	rosterFlagMaxX = 95  // position / captaincy markers ("G", "C", "A", "AP")
	rosterNameMaxX = 205 // player name (and trailing markers such as "(INJ)")
	penaltyMaxX    = 405 // penalty table lives between the roster and scoring columns
	shotsMinX      = 300 // SHOTS table (goalie, per-period, total, minutes)
)

var (
	reInt         = regexp.MustCompile(`^\d+$`)
	reFloat       = regexp.MustCompile(`^\d+(\.\d+)?$`)
	reTime        = regexp.MustCompile(`^\d{1,3}:\d{2}$`)
	rePeriod      = regexp.MustCompile(`^(\d+(st|nd|rd|th)|OT\d*|SO)$`)
	reGoalPer     = regexp.MustCompile(`^(\d+|OT\d*|SO)$`)
	rePenaltyCode = regexp.MustCompile(`^[A-Z]{2,6}(-[A-Z0-9]{1,4})+$`)
)

// Parse parses a scoresheet PDF.
func Parse(data []byte) (*Scoresheet, error) {
	pages, width, err := extractRuns(data)
	if err != nil {
		return nil, err
	}
	sheet := &Scoresheet{}
	for i, runs := range pages {
		if err := parsePage(sheet, runs, width, i == 0); err != nil {
			return nil, fmt.Errorf("page %d: %w", i+1, err)
		}
	}
	if sheet.Home.Name == "" && sheet.Visitor.Name == "" {
		return nil, fmt.Errorf("could not find team names; is this a GameSheet scoresheet?")
	}
	sheet.GameType = gameType(data)
	return sheet, nil
}

// parsePage parses one page and merges it into sheet.
func parsePage(sheet *Scoresheet, runs []run, width float64, first bool) error {
	// Locate the HOME / VISITOR team header row to find the split between the
	// two halves and the horizontal offset of the right half.
	var homeX, visX float64 = -1, -1
	for _, row := range groupRows(runs, 1.0) {
		var h, v *run
		for i := range row {
			switch row[i].S {
			case "HOME":
				if h == nil {
					h = &row[i]
				}
			case "VISITOR":
				if v == nil {
					v = &row[i]
				}
			}
		}
		// The team header row has HOME near the left margin; the score-box
		// header further up has HOME/VISITOR side by side near the centre.
		if h != nil && v != nil && h.X < 100 && v.X-h.X > width/3 {
			homeX, visX = h.X, v.X
			break
		}
	}
	if homeX < 0 {
		if !first {
			// Continuation pages without headers: assume the same geometry.
			homeX, visX = 39, 39+width/2-22
		} else {
			return fmt.Errorf("HOME/VISITOR headers not found")
		}
	}
	offset := visX - homeX
	splitX := visX - 8

	var left, right []run
	for _, rn := range runs {
		if rn.X < splitX {
			left = append(left, rn)
		} else {
			right = append(right, run{X: rn.X - offset, Y: rn.Y, S: rn.S})
		}
	}

	if first {
		parseGameInfo(sheet, runs)
	}
	parseSide(&sheet.Home, left)
	parseSide(&sheet.Visitor, right)
	return nil
}

// parseGameInfo pulls the game-level fields from the header boxes.
func parseGameInfo(sheet *Scoresheet, runs []run) {
	rows := groupRows(runs, 1.0)
	farRight := width80(rows)
	labelValue := func(row []run, label string) string {
		for i := 0; i+1 < len(row); i++ {
			if row[i].S == label {
				return row[i+1].S
			}
		}
		return ""
	}
	for _, row := range rows {
		for _, rn := range row {
			switch rn.S {
			case "FINAL", "IN PROGRESS", "SCHEDULED", "NOT STARTED":
				if sheet.Status == "" {
					sheet.Status = rn.S
				}
			}
		}
		if v := labelValue(row, "No:"); v != "" && sheet.GameNumber == "" {
			sheet.GameNumber = v
		}
		if v := labelValue(row, "Date"); v != "" && sheet.Date == "" {
			sheet.Date = v
		}
		if v := labelValue(row, "Location of Game"); v != "" && sheet.Location == "" {
			sheet.Location = v
		}
		if v := labelValue(row, "Division"); v != "" && sheet.Division == "" && v != "Category" {
			sheet.Division = v
		}
		if v := labelValue(row, "League"); v != "" && sheet.League == "" && v != "PLAYOFF GAME" {
			sheet.League = v
		}
		// Period lengths: "1st" "12" "Stop" in the Length of Game box.
		for i := 0; i+1 < len(row); i++ {
			if rePeriod.MatchString(row[i].S) && row[i].X > farRight && reFloat.MatchString(row[i+1].S) {
				m, _ := strconv.ParseFloat(row[i+1].S, 64)
				sheet.Periods = append(sheet.Periods, Period{Name: row[i].S, Minutes: m})
			}
		}
	}
}

// width80 returns an X threshold (80% of the right-most text) used to
// recognise the "Length of Game" box, which is at the far right of the page.
func width80(rows [][]run) float64 {
	var max float64
	for _, row := range rows {
		if x := row[len(row)-1].X; x > max {
			max = x
		}
	}
	return max * 0.8
}

// parseSide parses one half of the sheet (already shifted into HOME coordinates).
func parseSide(t *TeamSheet, runs []run) {
	rows := groupRows(runs, 1.0)

	// --- Landmarks ---------------------------------------------------------
	var (
		teamHeaderY  = -1.0 // "HOME"/"VISITOR" label above the team name
		rosterTopY   = -1.0 // "NO." header
		rosterBotY   = -1.0 // "List Suspended Players..." footer
		shotsHeaderY = -1.0 // "1 2 3 TOTAL" header row of the SHOTS table
		shotsBotY    = -1.0 // "Referee" row (end of SHOTS table)
		scoreAnchors []float64
		penAnchors   []float64
		penHeaderY   = -1.0
		shotAnchors  []float64
		shotLabels   []string
		goalieX      = -1.0
		minutesX     = -1.0
		goalsLabelY  = -1.0
		goalsLabelX  = -1.0
	)
	for _, row := range rows {
		y := row[0].Y
		for _, rn := range row {
			switch {
			case (rn.S == "HOME" || rn.S == "VISITOR") && rn.X < 100:
				// Several HOME/VISITOR labels exist (score box, team header,
				// totals box); the team header is the lowest one that is
				// still above the roster.
				if rosterTopY < 0 || y > rosterTopY {
					teamHeaderY = y
				}
			case rn.S == "NO." && rn.X < rosterNumMaxX && rosterTopY < 0:
				rosterTopY = y
			case strings.HasPrefix(rn.S, "List Suspended") && rosterBotY < 0:
				rosterBotY = y
			case rn.S == "GOALIE" && rn.X > shotsMinX:
				goalieX = rn.X
			case rn.S == "MINUTES" && rn.X > shotsMinX:
				minutesX = rn.X
			case rn.S == "Goals" && rn.X > penaltyMaxX:
				goalsLabelY, goalsLabelX = y, rn.X
			case rn.S == "Referee" && shotsBotY < 0:
				shotsBotY = y
			}
		}
		// Scoring header: PER TIME G A A in the scoring band.
		if scoreAnchors == nil {
			var xs []float64
			var labels []string
			for _, rn := range row {
				if rn.X > penaltyMaxX {
					xs = append(xs, rn.X)
					labels = append(labels, rn.S)
				}
			}
			if strings.Join(labels, " ") == "PER TIME G A A" {
				scoreAnchors = xs
			}
		}
		// Penalty header: PER NO. MIN. Code Infraction OFF START ON.
		if penAnchors == nil {
			var xs []float64
			var labels []string
			for _, rn := range row {
				if rn.X > rosterNameMaxX-10 && rn.X < penaltyMaxX {
					xs = append(xs, rn.X)
					labels = append(labels, rn.S)
				}
			}
			if strings.Contains(strings.Join(labels, " "), "Code Infraction") && len(xs) >= 4 {
				penAnchors, penHeaderY = xs, y
			}
		}
		// Shots header: 1 2 3 [OT] TOTAL in the shots band.
		if shotAnchors == nil {
			var xs []float64
			var labels []string
			for _, rn := range row {
				if rn.X > shotsMinX && rn.X < penaltyMaxX+100 {
					xs = append(xs, rn.X)
					labels = append(labels, rn.S)
				}
			}
			if len(labels) >= 2 && labels[len(labels)-1] == "TOTAL" && allPeriodLabels(labels[:len(labels)-1]) {
				shotAnchors, shotLabels, shotsHeaderY = xs, labels, y
			}
		}
	}

	// --- Team name & goals ---------------------------------------------------
	if teamHeaderY >= 0 {
		for _, row := range rows {
			y := row[0].Y
			// The team name sits between the HOME/VISITOR label and the
			// "NO." roster header (skip the "AP — AFFILIATED..." legend).
			if y < teamHeaderY && (rosterTopY < 0 || y > rosterTopY+5) && row[0].X < 100 {
				t.Name = row[0].S
				break
			}
		}
	}
	if goalsLabelY >= 0 {
		for _, rn := range runs {
			if abs(rn.Y-goalsLabelY) < 8 && rn.X > goalsLabelX+15 && reInt.MatchString(rn.S) {
				t.Goals, _ = strconv.Atoi(rn.S)
				break
			}
		}
	}

	// --- Roster & scoring plays --------------------------------------------
	if rosterTopY >= 0 && rosterBotY >= 0 {
		var rosterRuns, scoreRuns []run
		for _, rn := range runs {
			if rn.Y >= rosterTopY-1 || rn.Y <= rosterBotY+1 {
				continue
			}
			switch {
			case rn.X < rosterNameMaxX:
				rosterRuns = append(rosterRuns, rn)
			case rn.X > penaltyMaxX:
				scoreRuns = append(scoreRuns, rn)
			}
		}
		for _, row := range groupRows(rosterRuns, 1.0) {
			p := Player{}
			for _, rn := range row {
				switch {
				case rn.X < rosterNumMaxX && reInt.MatchString(rn.S) && p.Number == "":
					p.Number = rn.S
				case rn.X < rosterFlagMaxX:
					for _, f := range strings.Fields(rn.S) {
						if f == "G" {
							p.Position = "G"
						} else {
							p.Flags = append(p.Flags, f)
						}
					}
				case p.Name == "":
					p.Name = rn.S
				default:
					p.Flags = append(p.Flags, rn.S)
				}
			}
			if p.Number != "" || p.Name != "" {
				t.Roster = append(t.Roster, p)
			}
		}
		if scoreAnchors != nil {
			for _, row := range groupRows(scoreRuns, 1.0) {
				g := Goal{}
				ok := false
				for _, rn := range row {
					switch nearest(scoreAnchors, rn.X, 9) {
					case 0:
						if reGoalPer.MatchString(rn.S) {
							g.Period = rn.S
						}
					case 1:
						if reTime.MatchString(rn.S) {
							g.Time = rn.S
							ok = true
						}
					case 2:
						g.Scorer = rn.S
					case 3, 4:
						g.Assists = append(g.Assists, rn.S)
					}
				}
				if ok || g.Scorer != "" {
					t.Scores = append(t.Scores, g)
				}
			}
		}
		if penAnchors != nil && rosterBotY >= 0 {
			var penRuns []run
			for _, rn := range runs {
				if rn.Y < penHeaderY-1 && rn.Y > rosterBotY+1 && rn.X > rosterNameMaxX-10 && rn.X < penaltyMaxX {
					penRuns = append(penRuns, rn)
				}
			}
			for _, row := range groupRows(penRuns, 1.0) {
				p := Penalty{}
				for _, rn := range row {
					idx := nearest(penAnchors, rn.X, 14)
					if idx < 0 || idx >= len(penAnchors) {
						continue
					}
					// Anchors are PER, NO., MIN., Code, OFF, START, ON, but a
					// scoresheet occasionally drops a column, so match on the
					// header text we stored alongside... we only kept X. The
					// header order is stable: 0 period, 1 player, 2 minutes,
					// 3 code, and the last three are off/start/on times.
					switch {
					case idx == 0 && reGoalPer.MatchString(rn.S):
						p.Period = rn.S
					case idx == 1 && reInt.MatchString(rn.S):
						p.Number = rn.S
					case idx == 2 && reFloat.MatchString(rn.S):
						p.Minutes, _ = strconv.ParseFloat(rn.S, 64)
						p.HasMinutes = true
					case idx == 3 && rePenaltyCode.MatchString(rn.S):
						p.Code = rn.S
					case idx >= len(penAnchors)-3 && reTime.MatchString(rn.S):
						switch idx - (len(penAnchors) - 3) {
						case 0:
							// OFF time: when the whistle blew. Not needed.
						case 1:
							p.Start = rn.S
						case 2:
							p.On = rn.S
						}
					}
				}
				if p.Code != "" {
					t.Penalties = append(t.Penalties, p)
				}
			}
		}
	}

	// --- Shots table -------------------------------------------------------
	if shotAnchors != nil && goalieX > 0 {
		t.ShotColumns = shotLabels[:len(shotLabels)-1]
		nPer := len(t.ShotColumns)
		totalIdx := len(shotAnchors) - 1
		var shotRuns []run
		for _, rn := range runs {
			if rn.Y < shotsHeaderY-1 && (shotsBotY < 0 || rn.Y > shotsBotY+1) && rn.X > shotsMinX {
				shotRuns = append(shotRuns, rn)
			}
		}
		for _, row := range groupRows(shotRuns, 1.0) {
			var label string
			shots := make([]int, nPer)
			var seen []bool = make([]bool, nPer)
			total, hasTotal := 0, false
			minutes := 0.0
			for _, rn := range row {
				switch {
				case abs(rn.X-goalieX) < 12:
					label = rn.S
				case minutesX > 0 && abs(rn.X-minutesX) < 15 && reFloat.MatchString(rn.S):
					minutes, _ = strconv.ParseFloat(rn.S, 64)
				default:
					idx := nearest(shotAnchors, rn.X, 9)
					if idx < 0 || !reInt.MatchString(rn.S) {
						continue
					}
					n, _ := strconv.Atoi(rn.S)
					if idx == totalIdx {
						total, hasTotal = n, true
					} else {
						shots[idx], seen[idx] = n, true
					}
				}
			}
			if label == "" {
				continue
			}
			hasShots := false
			for _, s := range seen {
				hasShots = hasShots || s
			}
			if !hasTotal && hasShots {
				for _, s := range shots {
					total += s
				}
			}
			if strings.EqualFold(label, "TOTAL") {
				t.ShotsFaced = shots
				t.ShotsFacedTotal = total
				t.MinutesTotal = minutes
				continue
			}
			if !reInt.MatchString(label) {
				continue
			}
			gl := GoalieLine{Number: label, HasShots: hasShots, Total: total, Minutes: minutes}
			if hasShots {
				gl.Shots = shots
			}
			t.Goalies = append(t.Goalies, gl)
		}
	}
}

func allPeriodLabels(labels []string) bool {
	for _, l := range labels {
		if !reGoalPer.MatchString(l) {
			return false
		}
	}
	return true
}
