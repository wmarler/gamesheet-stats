package stats

import (
	"sort"
	"strings"
)

// Summary aggregates GameStats across games.
type Summary struct {
	Games        int
	Wins, Losses int
	Ties         int
	GoalsFor     int
	GoalsAgt     int
	ShotsFor     int
	ShotsAgt     int
	Goalies      []GoalieTotals
	Players      []PlayerTotals
}

// GoalieTotals is a goalie's season line.
type GoalieTotals struct {
	Name      string
	Numbers   []string
	Games     int // games with recorded shots/minutes
	Shots     int
	Minutes   float64
	Estimated bool // some games used prorated shot counts
}

// PlayerTotals is a skater's (or goalie's) season scoring line.
type PlayerTotals struct {
	Name    string
	Numbers []string
	Games   int
	Goals   int
	Assists int
}

func (p PlayerTotals) Points() int { return p.Goals + p.Assists }

// Summarize aggregates per-game stats. Players are matched across games by
// name (jersey numbers can change between leagues/seasons).
func Summarize(games []*GameStats) Summary {
	var s Summary
	goalies := map[string]*GoalieTotals{}
	players := map[string]*PlayerTotals{}
	for _, g := range games {
		s.Games++
		switch g.Result {
		case "W":
			s.Wins++
		case "L":
			s.Losses++
		default:
			s.Ties++
		}
		s.GoalsFor += g.GoalsFor
		s.GoalsAgt += g.GoalsAgt
		s.ShotsFor += g.ShotsForTot
		s.ShotsAgt += g.ShotsAgtTot
		for _, gl := range g.Goalies {
			if gl.Dressed {
				continue
			}
			key := playerKey(gl.Name, gl.Number)
			t := goalies[key]
			if t == nil {
				t = &GoalieTotals{Name: gl.Name}
				goalies[key] = t
			}
			t.Numbers = addUnique(t.Numbers, gl.Number)
			t.Games++
			t.Shots += gl.Total
			t.Minutes += gl.Minutes
			t.Estimated = t.Estimated || gl.Estimated
		}
		for _, p := range g.Players {
			key := playerKey(p.Name, p.Number)
			t := players[key]
			if t == nil {
				t = &PlayerTotals{Name: p.Name}
				players[key] = t
			}
			t.Numbers = addUnique(t.Numbers, p.Number)
			t.Games++
			t.Goals += p.Goals
			t.Assists += p.Assists
		}
	}
	for _, t := range goalies {
		s.Goalies = append(s.Goalies, *t)
	}
	sort.Slice(s.Goalies, func(i, j int) bool { return s.Goalies[i].Shots > s.Goalies[j].Shots })
	for _, t := range players {
		s.Players = append(s.Players, *t)
	}
	sort.Slice(s.Players, func(i, j int) bool {
		a, b := s.Players[i], s.Players[j]
		if a.Points() != b.Points() {
			return a.Points() > b.Points()
		}
		if a.Goals != b.Goals {
			return a.Goals > b.Goals
		}
		return a.Name < b.Name
	})
	return s
}

func playerKey(name, number string) string {
	if n := strings.TrimSpace(strings.ToLower(name)); n != "" {
		return n
	}
	return "#" + number
}

func addUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}
