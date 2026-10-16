package stats

import (
	"reflect"
	"testing"

	"github.com/wmarler/gamesheet-stats/internal/gamesheet"
	"github.com/wmarler/gamesheet-stats/internal/scoresheet"
)

func intp(i int) *int { return &i }

func sampleSheet() *scoresheet.Scoresheet {
	return &scoresheet.Scoresheet{
		GameNumber: "32",
		League:     "Example League",
		Home: scoresheet.TeamSheet{
			Name:  "Opponents",
			Goals: 2,
			Roster: []scoresheet.Player{
				{Number: "1", Name: "OPP GOALIE", Position: "G"},
				{Number: "20", Name: "OPP SCORER"},
			},
			Scores:          []scoresheet.Goal{{Period: "1", Time: "08:57", Scorer: "20"}, {Period: "2", Time: "09:21", Scorer: "26", Assists: []string{"88"}}},
			ShotColumns:     []string{"1", "2", "3"},
			Goalies:         []scoresheet.GoalieLine{{Number: "1", Shots: []int{4, 3, 2}, HasShots: true, Total: 9, Minutes: 36}},
			ShotsFaced:      []int{4, 3, 2},
			ShotsFacedTotal: 9,
			MinutesTotal:    36,
		},
		Visitor: scoresheet.TeamSheet{
			Name:  "Our Team",
			Goals: 1,
			Roster: []scoresheet.Player{
				{Number: "1", Name: "FIRST GOALIE", Position: "G"},
				{Number: "31", Name: "SECOND GOALIE", Position: "G"},
				{Number: "3", Name: "GOAL SCORER"},
				{Number: "19", Name: "HELPER MCPLAYER"},
				{Number: "5", Name: "QUIET SKATER"},
			},
			Scores:      []scoresheet.Goal{{Period: "3", Time: "09:11", Scorer: "3", Assists: []string{"19"}}},
			ShotColumns: []string{"1", "2", "3"},
			Goalies: []scoresheet.GoalieLine{
				{Number: "1", Shots: []int{2, 3, 0}, HasShots: true, Total: 5, Minutes: 18.17},
				{Number: "31", Shots: []int{0, 2, 2}, HasShots: true, Total: 4, Minutes: 17.83},
			},
			ShotsFaced:      []int{2, 5, 2},
			ShotsFacedTotal: 9,
			MinutesTotal:    36,
		},
	}
}

func sampleGame() gamesheet.Game {
	g := gamesheet.Game{ID: "3001590", EventDate: "2026-09-19", EventLocation: "KHS"}
	g.EventData.IsHome = false
	g.EventData.HomeGoals = intp(2)
	g.EventData.VisitorGoals = intp(1)
	g.EventData.SeasonID = 15755
	return g
}

func TestBuild(t *testing.T) {
	gs, err := Build(sampleGame(), sampleSheet())
	if err != nil {
		t.Fatal(err)
	}
	if gs.TeamName != "Our Team" || gs.Opponent != "Opponents" || gs.IsHome {
		t.Errorf("teams: %+v", gs)
	}
	if gs.GoalsFor != 1 || gs.GoalsAgt != 2 || gs.Result != "L" {
		t.Errorf("score: %d-%d %s", gs.GoalsFor, gs.GoalsAgt, gs.Result)
	}
	if gs.ShotsForTot != 9 || !reflect.DeepEqual(gs.ShotsFor, []int{4, 3, 2}) {
		t.Errorf("shots for: %d %v", gs.ShotsForTot, gs.ShotsFor)
	}
	if gs.ShotsAgtTot != 9 || !reflect.DeepEqual(gs.ShotsAgt, []int{2, 5, 2}) {
		t.Errorf("shots against: %d %v", gs.ShotsAgtTot, gs.ShotsAgt)
	}
	if len(gs.Goalies) != 2 || gs.Goalies[0].Name != "First Goalie" || gs.Goalies[0].Total != 5 || gs.Goalies[1].Total != 4 || gs.Goalies[0].Estimated {
		t.Errorf("goalies: %+v", gs.Goalies)
	}
	if len(gs.Goals) != 1 || gs.Goals[0].Scorer.Name != "Goal Scorer" || gs.Goals[0].Assists[0].Name != "Helper McPlayer" {
		t.Errorf("goals: %+v", gs.Goals)
	}
	if len(gs.Against) != 2 || gs.Against[0].Scorer.Name != "Opp Scorer" || gs.Against[1].Scorer.Name != "" {
		t.Errorf("against: %+v", gs.Against)
	}
	// Players sorted by points; everyone dressed is included.
	if len(gs.Players) != 5 {
		t.Fatalf("players: %+v", gs.Players)
	}
	if gs.Players[0].Name != "Goal Scorer" || gs.Players[0].Goals != 1 || gs.Players[1].Name != "Helper McPlayer" || gs.Players[1].Assists != 1 {
		t.Errorf("player order: %+v", gs.Players)
	}
	if len(gs.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", gs.Warnings)
	}
}

func TestBuildProratesGoalieWithoutShots(t *testing.T) {
	sheet := sampleSheet()
	// Second goalie's row has minutes only: 17.83 of 36 minutes.
	sheet.Visitor.Goalies[1] = scoresheet.GoalieLine{Number: "31", Minutes: 17.83}
	// Team totals as before (2-5-2); goalie #1 has 2-3-0 known.
	gs, err := Build(sampleGame(), sheet)
	if err != nil {
		t.Fatal(err)
	}
	g := gs.Goalies[1]
	if !g.Estimated {
		t.Fatalf("expected estimate, got %+v", g)
	}
	// Remaining shots per period: 0, 2, 2 — only one unknown goalie, so it gets all of them.
	if !reflect.DeepEqual(g.Shots, []int{0, 2, 2}) || g.Total != 4 {
		t.Errorf("prorated shots = %v total %d", g.Shots, g.Total)
	}
	if len(gs.Warnings) == 0 {
		t.Errorf("expected a warning about estimation")
	}
}

func TestBuildProratesTwoGoaliesByMinutes(t *testing.T) {
	sheet := sampleSheet()
	sheet.Visitor.Goalies = []scoresheet.GoalieLine{
		{Number: "1", Minutes: 24},  // 2/3 of the time
		{Number: "31", Minutes: 12}, // 1/3 of the time
	}
	sheet.Visitor.ShotsFaced = []int{3, 6, 9}
	sheet.Visitor.ShotsFacedTotal = 18
	gs, err := Build(sampleGame(), sheet)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gs.Goalies[0].Shots, []int{2, 4, 6}) || gs.Goalies[0].Total != 12 {
		t.Errorf("goalie 1 = %+v", gs.Goalies[0])
	}
	if !reflect.DeepEqual(gs.Goalies[1].Shots, []int{1, 2, 3}) || gs.Goalies[1].Total != 6 {
		t.Errorf("goalie 2 = %+v", gs.Goalies[1])
	}
}

func TestBuildUsesCalendarScoreOnMismatch(t *testing.T) {
	sheet := sampleSheet()
	sheet.Visitor.Goals = 7 // bogus
	gs, err := Build(sampleGame(), sheet)
	if err != nil {
		t.Fatal(err)
	}
	if gs.GoalsFor != 1 || len(gs.Warnings) == 0 {
		t.Errorf("GoalsFor = %d warnings=%v", gs.GoalsFor, gs.Warnings)
	}
}

func TestSummarize(t *testing.T) {
	g1, _ := Build(sampleGame(), sampleSheet())
	// Second game: we are home, win 3-0, same players with a different jersey for the scorer.
	sheet := sampleSheet()
	sheet.Home, sheet.Visitor = sheet.Visitor, sheet.Home
	sheet.Home.Goals, sheet.Visitor.Goals = 3, 0
	sheet.Home.Roster[2].Number = "4"
	sheet.Home.Scores = []scoresheet.Goal{
		{Period: "1", Time: "01:00", Scorer: "4", Assists: []string{"19", "5"}},
		{Period: "2", Time: "01:00", Scorer: "4"},
		{Period: "3", Time: "01:00", Scorer: "19", Assists: []string{"4"}},
	}
	sheet.Visitor.Scores = nil
	game2 := sampleGame()
	game2.ID = "2"
	game2.EventData.IsHome = true
	game2.EventData.HomeGoals, game2.EventData.VisitorGoals = intp(3), intp(0)
	g2, err := Build(game2, sheet)
	if err != nil {
		t.Fatal(err)
	}

	s := Summarize([]*GameStats{g1, g2})
	if s.Games != 2 || s.Wins != 1 || s.Losses != 1 || s.GoalsFor != 4 || s.GoalsAgt != 2 || s.ShotsFor != 18 || s.ShotsAgt != 18 {
		t.Errorf("summary = %+v", s)
	}
	if s.Players[0].Name != "Goal Scorer" || s.Players[0].Goals != 3 || s.Players[0].Assists != 1 || !reflect.DeepEqual(s.Players[0].Numbers, []string{"3", "4"}) || s.Players[0].Games != 2 {
		t.Errorf("top player = %+v", s.Players[0])
	}
	if len(s.Goalies) != 2 || s.Goalies[0].Shots != 10 || s.Goalies[0].Games != 2 {
		t.Errorf("goalies = %+v", s.Goalies)
	}
}

func TestPrettyName(t *testing.T) {
	cases := map[string]string{
		"SASHA SPEIL":      "Sasha Speil",
		"GREGORY ROWE II":  "Gregory Rowe II",
		"MARY-KATE O'NEIL": "Mary-Kate O'Neil",
		"JOHN MCDONALD":    "John McDonald",
		"":                 "",
	}
	for in, want := range cases {
		if got := PrettyName(in); got != want {
			t.Errorf("PrettyName(%q) = %q, want %q", in, got, want)
		}
	}
}
