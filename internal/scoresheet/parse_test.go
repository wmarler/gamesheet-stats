package scoresheet

import (
	"reflect"
	"testing"
)

// sideRuns builds a synthetic set of text runs for one half of a scoresheet,
// using the X/Y positions observed in real GameSheet PDFs (HOME-half
// coordinates). Names are fictional.
func sideRuns() []run {
	return []run{
		// Score box label (must be ignored when finding the team header).
		{469, 583, "HOME"},
		// Team header and name.
		{39, 487, "HOME"},
		{39, 478, "Example Hockey Club 12U"},
		// Column headers.
		{212, 475.5, "PER"}, {232, 475.5, "NO."}, {250, 475.5, "MIN."}, {271, 475.5, "Code Infraction"},
		{330, 475.5, "OFF"}, {355, 475.5, "START"}, {390, 475.5, "ON"},
		{415, 475.5, "PER"}, {437, 475.5, "TIME"}, {465, 475.5, "G"}, {484, 475.5, "A"}, {502, 475.5, "A"},
		{61, 464.5, "AP — AFFILIATED PLAYER C — CAPTAIN A — ALTERNATE"},
		{37, 463.5, "NO."},
		// Roster.
		{43, 451.5, "1"}, {60, 451.5, "G"}, {110, 451.5, "ALPHA GOALIE"},
		{41, 439.5, "30"}, {60, 439.5, "G"}, {108, 439.5, "BETA GOALIE"},
		{41, 427.5, "10"}, {60, 427.5, "C"}, {105, 427.5, "CAPTAIN SKATER"},
		{41, 415.5, "11"}, {112, 415.5, "WINGER ONE"}, {189, 415.5, "(INJ)"},
		{43, 403.5, "7"}, {110, 403.5, "DEFENDER TWO"},
		// A penalty row (should be ignored).
		{218, 450, "2"}, {234, 450, "7"}, {256, 450, "2"}, {283, 450, "TR-MIN"}, {328, 450, "07:09"}, {357, 450, "07:09"}, {386, 450, "05:21"},
		// Scoring plays.
		{420, 462, "1"}, {436, 462, "08:57"}, {464, 462, "10"},
		{420, 450, "2"}, {436, 450, "09:21"}, {464, 450, "11"}, {482, 450, "10"},
		{420, 438, "3"}, {436, 438, "00:39"}, {466, 438, "7"}, {484, 438, "10"}, {500, 438, "11"},
		// Roster footer.
		{58, 210.5, "List Suspended Players/Officials NO. of Games"},
		// Totals box.
		{271, 175.5, "Total"}, {443, 168.5, "Total"}, {340, 166, "HOME"},
		{235, 165.5, "2"}, {482, 165.5, "3"},
		{271, 162, "Minutes"}, {440, 162, "Goals"},
		// Shots table.
		{40, 146.5, "Head Coach"}, {162, 146.5, "COACH NAME"}, {358, 146.5, "SHOTS"},
		{448, 143, "MINUTES"}, {314, 139.5, "GOALIE"}, {448, 136.5, "PLAYED"},
		{40, 132.5, "Trainer"}, {358, 132.5, "1"}, {376, 132.5, "2"}, {394, 132.5, "3"}, {412, 132.5, "TOTAL"},
		{40, 118.5, "Manager, Asst. Coach or Asst. Trainer"}, {176, 118.5, "HELPER ONE"},
		{314, 118.5, "1"}, {358, 118.5, "2"}, {376, 118.5, "3"}, {394, 118.5, "0"}, {412, 118.5, "5"}, {448, 118.5, "18.17"},
		{40, 104.5, "Manager, Asst. Coach or Asst. Trainer"}, {170, 104.5, "HELPER TWO"},
		{314, 104.5, "30"}, {358, 104.5, "0"}, {376, 104.5, "2"}, {394, 104.5, "2"}, {412, 104.5, "4"}, {448, 104.5, "17.83"},
		{40, 90.5, "Manager, Asst. Coach or Asst. Trainer"},
		{314, 90.5, "TOTAL"}, {358, 90.5, "2"}, {376, 90.5, "5"}, {394, 90.5, "2"}, {412, 90.5, "9"}, {448, 90.5, "36.00"},
		// Officials (below the shots table).
		{274, 73.5, "Referee"}, {416, 71.5, "REF NAME"},
	}
}

func TestParseSide(t *testing.T) {
	var ts TeamSheet
	parseSide(&ts, sideRuns())

	if ts.Name != "Example Hockey Club 12U" {
		t.Errorf("Name = %q", ts.Name)
	}
	if ts.Goals != 3 {
		t.Errorf("Goals = %d, want 3", ts.Goals)
	}

	wantRoster := []Player{
		{Number: "1", Name: "ALPHA GOALIE", Position: "G"},
		{Number: "30", Name: "BETA GOALIE", Position: "G"},
		{Number: "10", Name: "CAPTAIN SKATER", Flags: []string{"C"}},
		{Number: "11", Name: "WINGER ONE", Flags: []string{"(INJ)"}},
		{Number: "7", Name: "DEFENDER TWO"},
	}
	if !reflect.DeepEqual(ts.Roster, wantRoster) {
		t.Errorf("Roster = %+v\nwant %+v", ts.Roster, wantRoster)
	}

	wantScores := []Goal{
		{Period: "1", Time: "08:57", Scorer: "10"},
		{Period: "2", Time: "09:21", Scorer: "11", Assists: []string{"10"}},
		{Period: "3", Time: "00:39", Scorer: "7", Assists: []string{"10", "11"}},
	}
	if !reflect.DeepEqual(ts.Scores, wantScores) {
		t.Errorf("Scores = %+v\nwant %+v", ts.Scores, wantScores)
	}

	if !reflect.DeepEqual(ts.ShotColumns, []string{"1", "2", "3"}) {
		t.Errorf("ShotColumns = %v", ts.ShotColumns)
	}
	if !reflect.DeepEqual(ts.ShotsFaced, []int{2, 5, 2}) || ts.ShotsFacedTotal != 9 || ts.MinutesTotal != 36 {
		t.Errorf("ShotsFaced = %v total=%d minutes=%v", ts.ShotsFaced, ts.ShotsFacedTotal, ts.MinutesTotal)
	}
	wantGoalies := []GoalieLine{
		{Number: "1", Shots: []int{2, 3, 0}, HasShots: true, Total: 5, Minutes: 18.17},
		{Number: "30", Shots: []int{0, 2, 2}, HasShots: true, Total: 4, Minutes: 17.83},
	}
	if !reflect.DeepEqual(ts.Goalies, wantGoalies) {
		t.Errorf("Goalies = %+v\nwant %+v", ts.Goalies, wantGoalies)
	}
}

func TestParseSideGoalieWithoutShots(t *testing.T) {
	runs := sideRuns()
	// Strip the per-period cells from goalie #30's row, leaving only minutes.
	var kept []run
	for _, r := range runs {
		if r.Y == 104.5 && r.X >= 358 && r.X <= 412 {
			continue
		}
		kept = append(kept, r)
	}
	var ts TeamSheet
	parseSide(&ts, kept)
	if len(ts.Goalies) != 2 {
		t.Fatalf("got %d goalies", len(ts.Goalies))
	}
	g := ts.Goalies[1]
	if g.Number != "30" || g.HasShots || g.Shots != nil || g.Minutes != 17.83 {
		t.Errorf("goalie row without shots = %+v", g)
	}
}

func TestGroupRows(t *testing.T) {
	rows := groupRows([]run{{10, 100, "a"}, {50, 100.4, "b"}, {5, 90, "c"}, {20, 90.9, "d"}}, 1.0)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[0][0].S != "a" || rows[0][1].S != "b" {
		t.Errorf("row 0 = %v", rows[0])
	}
	if rows[1][0].S != "c" || rows[1][1].S != "d" {
		t.Errorf("row 1 = %v", rows[1])
	}
}

func TestNearest(t *testing.T) {
	anchors := []float64{415, 437, 465, 484, 502}
	cases := map[float64]int{420: 0, 436: 1, 466: 2, 482: 3, 500: 4, 600: -1}
	for x, want := range cases {
		if got := nearest(anchors, x, 9); got != want {
			t.Errorf("nearest(%v) = %d, want %d", x, got, want)
		}
	}
}
