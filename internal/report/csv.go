package report

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wmarler/gamesheet-stats/internal/stats"
)

// WriteCSVs writes the three log files into dir:
//
//	game_log.csv     one row per game
//	goal_log.csv     one row per goal we scored
//	penalty_log.csv  one row per penalty we took
//
// A cell is left empty when that fact cannot be read from the scoresheet.
// It returns the paths written.
func WriteCSVs(dir string, games []*stats.GameStats) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	// These were produced by an earlier version of the program.
	for _, old := range []string{"games.csv", "goalies.csv", "players.csv", "goals.csv"} {
		_ = os.Remove(filepath.Join(dir, old))
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

	gameHeader := []string{
		"Game #", "Date", "Opponent", "Home/Away", "Game Type",
		"Goals-For", "Shots-For",
		"Goalie 1", "Goalie 1 Minutes", "Goalie 1 Shots Faced", "Goalie 1 Goals Against",
		"Goalie 2", "Goalie 2 Minutes", "Goalie 2 Shots Faced", "Goalie 2 Goals Against",
		"Empty-net Goals Against", "How the game ended",
	}
	var gameRows, goalRows, penRows [][]string
	for i, g := range games {
		// Game # is the row's place in this log, not the GameSheet id.
		id := strconv.Itoa(i + 1)
		ha := "Home"
		if !g.IsHome {
			ha = "Away"
		}
		row := []string{
			id, g.Date, g.Opponent, ha, g.GameType,
			strconv.Itoa(g.GoalsFor), shotsCell(g.ShotsForTot, g.ShotsFor != nil),
		}
		played := playedGoalies(g)
		for slot := 0; slot < 2; slot++ {
			if slot >= len(played) {
				row = append(row, "", "", "", "")
				continue
			}
			gl := played[slot]
			ga := ""
			if gl.GoalsAgainstKnown {
				ga = strconv.Itoa(gl.GoalsAgainst)
			}
			row = append(row, goalieName(gl), formatMinutes(gl.Minutes), strconv.Itoa(gl.Total), ga)
		}
		en := ""
		if g.EmptyNetKnown {
			en = strconv.Itoa(g.EmptyNetAgainst)
		}
		row = append(row, en, g.Ended)
		gameRows = append(gameRows, row)

		for _, ev := range g.Goals {
			a1, a2 := "", ""
			if len(ev.Assists) > 0 {
				a1 = personName(ev.Assists[0])
			}
			if len(ev.Assists) > 1 {
				a2 = personName(ev.Assists[1])
			}
			goalRows = append(goalRows, []string{id, ev.Period, personName(ev.Scorer), a1, a2, ev.Situation})
		}
		for _, p := range g.Penalties {
			penRows = append(penRows, []string{id, p.Period, personName(p.Player), p.Type, p.Minutes})
		}
	}

	if err := write("game_log.csv", gameHeader, gameRows); err != nil {
		return nil, err
	}
	if err := write("goal_log.csv", []string{"Game #", "Period", "Scorer", "Assist 1", "Assist 2", "Situation"}, goalRows); err != nil {
		return nil, err
	}
	if err := write("penalty_log.csv", []string{"Game #", "Period", "Player", "Penalty Type", "Minutes"}, penRows); err != nil {
		return nil, err
	}
	return paths, nil
}

func playedGoalies(g *stats.GameStats) []stats.GoalieGame {
	var out []stats.GoalieGame
	for _, gl := range g.Goalies {
		if gl.Dressed {
			continue
		}
		out = append(out, gl)
	}
	return out
}

func goalieName(g stats.GoalieGame) string {
	if g.Name != "" {
		return g.Name
	}
	if g.Number != "" {
		return "#" + g.Number
	}
	return ""
}

func personName(p stats.Person) string {
	if p.Name != "" {
		return p.Name
	}
	if p.Number != "" {
		return "#" + p.Number
	}
	return ""
}

func shotsCell(n int, known bool) string {
	if !known {
		return ""
	}
	return strconv.Itoa(n)
}

func formatMinutes(m float64) string {
	s := strconv.FormatFloat(m, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}
