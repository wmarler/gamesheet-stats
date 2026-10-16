package report

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wmarler/gamesheet-stats/internal/stats"
)

// WriteCSVs writes four CSV files into dir:
//
//	games.csv    one row per game (team-level: score, shots for/against by period)
//	goalies.csv  one row per goalie per game (shots faced by period, minutes)
//	players.csv  one row per dressed player per game (goals, assists)
//	goals.csv    one row per scoring play (scorer, assists)
//
// It returns the paths written.
func WriteCSVs(dir string, games []*stats.GameStats) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	maxPer := 0
	for _, g := range games {
		if n := len(g.PeriodLabels); n > maxPer {
			maxPer = n
		}
	}
	if maxPer == 0 {
		maxPer = 3
	}
	perCols := func(prefix string) []string {
		out := make([]string, maxPer)
		for i := range out {
			out[i] = fmt.Sprintf("%s_p%d", prefix, i+1)
		}
		return out
	}
	perVals := func(vals []int) []string {
		out := make([]string, maxPer)
		for i := range out {
			if i < len(vals) {
				out[i] = strconv.Itoa(vals[i])
			}
		}
		return out
	}
	gameCols := []string{"game_id", "date", "season_id", "league", "game_no", "team", "opponent", "home_away", "result", "goals_for", "goals_against"}
	gameVals := func(g *stats.GameStats) []string {
		ha := "home"
		if !g.IsHome {
			ha = "away"
		}
		return []string{g.GameID, g.Date, strconv.Itoa(g.SeasonID), g.League, g.GameNo, g.TeamName, g.Opponent, ha, g.Result, strconv.Itoa(g.GoalsFor), strconv.Itoa(g.GoalsAgt)}
	}

	var paths []string
	write := func(name string, header []string, rows [][]string) error {
		p := filepath.Join(dir, name)
		f, err := os.Create(p)
		if err != nil {
			return err
		}
		defer f.Close()
		cw := csv.NewWriter(f)
		if err := cw.Write(header); err != nil {
			return err
		}
		for _, r := range rows {
			if err := cw.Write(r); err != nil {
				return err
			}
		}
		cw.Flush()
		paths = append(paths, p)
		return cw.Error()
	}

	// games.csv
	{
		header := append(append([]string{}, gameCols...), "shots_for")
		header = append(header, perCols("shots_for")...)
		header = append(header, "shots_against")
		header = append(header, perCols("shots_against")...)
		header = append(header, "location", "warnings")
		var rows [][]string
		for _, g := range games {
			r := append(gameVals(g), strconv.Itoa(g.ShotsForTot))
			r = append(r, perVals(g.ShotsFor)...)
			r = append(r, strconv.Itoa(g.ShotsAgtTot))
			r = append(r, perVals(g.ShotsAgt)...)
			r = append(r, g.Location, strings.Join(g.Warnings, "; "))
			rows = append(rows, r)
		}
		if err := write("games.csv", header, rows); err != nil {
			return nil, err
		}
	}

	// goalies.csv
	{
		header := append(append([]string{}, gameCols...), "goalie_no", "goalie", "shots_faced")
		header = append(header, perCols("shots_faced")...)
		header = append(header, "minutes", "estimated", "dressed_only")
		var rows [][]string
		for _, g := range games {
			for _, gl := range g.Goalies {
				r := append(gameVals(g), gl.Number, gl.Name)
				if gl.Dressed {
					r = append(r, "")
					r = append(r, perVals(nil)...)
					r = append(r, "", "false", "true")
				} else {
					r = append(r, strconv.Itoa(gl.Total))
					r = append(r, perVals(gl.Shots)...)
					r = append(r, strconv.FormatFloat(gl.Minutes, 'f', 2, 64), strconv.FormatBool(gl.Estimated), "false")
				}
				rows = append(rows, r)
			}
		}
		if err := write("goalies.csv", header, rows); err != nil {
			return nil, err
		}
	}

	// players.csv
	{
		header := append(append([]string{}, gameCols...), "player_no", "player", "position", "goals", "assists", "points")
		var rows [][]string
		for _, g := range games {
			for _, p := range g.Players {
				rows = append(rows, append(gameVals(g), p.Number, p.Name, p.Position, strconv.Itoa(p.Goals), strconv.Itoa(p.Assists), strconv.Itoa(p.Points())))
			}
		}
		if err := write("players.csv", header, rows); err != nil {
			return nil, err
		}
	}

	// goals.csv
	{
		header := append(append([]string{}, gameCols...), "scoring_team", "period", "time", "scorer_no", "scorer", "assist1_no", "assist1", "assist2_no", "assist2")
		var rows [][]string
		add := func(g *stats.GameStats, who string, evs []stats.GoalEvent) {
			for _, ev := range evs {
				r := append(gameVals(g), who, ev.Period, ev.Time, ev.Scorer.Number, ev.Scorer.Name)
				for i := 0; i < 2; i++ {
					if i < len(ev.Assists) {
						r = append(r, ev.Assists[i].Number, ev.Assists[i].Name)
					} else {
						r = append(r, "", "")
					}
				}
				rows = append(rows, r)
			}
		}
		for _, g := range games {
			add(g, "us", g.Goals)
			add(g, "opponent", g.Against)
		}
		if err := write("goals.csv", header, rows); err != nil {
			return nil, err
		}
	}
	return paths, nil
}
