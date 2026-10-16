// Package report renders GameStats as human-readable text and as CSV files.
package report

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/wmarler/gamesheet-stats/internal/stats"
)

// WriteText prints a per-game report followed by season totals.
func WriteText(w io.Writer, games []*stats.GameStats) {
	for i, g := range games {
		if i > 0 {
			fmt.Fprintln(w)
		}
		writeGame(w, g)
	}
	if len(games) == 0 {
		fmt.Fprintln(w, "No completed games found.")
		return
	}
	fmt.Fprintln(w)
	writeSummary(w, stats.Summarize(games))
}

func writeGame(w io.Writer, g *stats.GameStats) {
	ha := "vs"
	if !g.IsHome {
		ha = "@"
	}
	fmt.Fprintf(w, "%s  %s %s  %s %d-%d\n", g.Date, ha, g.Opponent, g.Result, g.GoalsFor, g.GoalsAgt)
	meta := []string{"game " + g.GameID}
	if g.GameNo != "" {
		meta = append(meta, "no. "+g.GameNo)
	}
	if g.League != "" {
		meta = append(meta, g.League)
	}
	if g.Location != "" {
		meta = append(meta, g.Location)
	}
	fmt.Fprintf(w, "  %s\n", strings.Join(meta, " | "))

	fmt.Fprintf(w, "  Shots for:     %s\n", shotsStr(g.ShotsForTot, g.ShotsFor, g.ShotsFor != nil))
	fmt.Fprintf(w, "  Shots against: %s\n", shotsStr(g.ShotsAgtTot, g.ShotsAgt, g.ShotsAgt != nil))

	if len(g.Goalies) > 0 {
		fmt.Fprintf(w, "  Goalies:\n")
		for _, gl := range g.Goalies {
			label := fmt.Sprintf("#%s %s", gl.Number, gl.Name)
			if gl.Dressed {
				fmt.Fprintf(w, "    %-26s dressed, did not play\n", label)
				continue
			}
			est := ""
			if gl.Estimated {
				est = "  (estimated from minutes played)"
			}
			fmt.Fprintf(w, "    %-26s %s, %.2f min%s\n", label, shotsStr(gl.Total, gl.Shots, true), gl.Minutes, est)
		}
	}

	if len(g.Goals) > 0 {
		fmt.Fprintf(w, "  Scoring:\n")
		for _, ev := range g.Goals {
			line := fmt.Sprintf("    P%s %s  %s", ev.Period, ev.Time, ev.Scorer)
			if len(ev.Assists) > 0 {
				var as []string
				for _, a := range ev.Assists {
					as = append(as, a.String())
				}
				line += "  (A: " + strings.Join(as, ", ") + ")"
			} else {
				line += "  (unassisted)"
			}
			fmt.Fprintln(w, line)
		}
	}

	var pts []string
	for _, p := range g.Players {
		if p.Points() == 0 {
			continue
		}
		pts = append(pts, fmt.Sprintf("#%s %s %dG %dA", p.Number, p.Name, p.Goals, p.Assists))
	}
	if len(pts) > 0 {
		fmt.Fprintf(w, "  Points:  %s\n", strings.Join(pts, "; "))
	}
	for _, wmsg := range g.Warnings {
		fmt.Fprintf(w, "  ! %s\n", wmsg)
	}
}

func writeSummary(w io.Writer, s stats.Summary) {
	fmt.Fprintf(w, "==== Season totals: %d games, %d-%d-%d (W-L-T) ====\n", s.Games, s.Wins, s.Losses, s.Ties)
	fmt.Fprintf(w, "  Goals for / against: %d / %d\n", s.GoalsFor, s.GoalsAgt)
	fmt.Fprintf(w, "  Shots for / against: %d / %d\n", s.ShotsFor, s.ShotsAgt)
	if len(s.Goalies) > 0 {
		fmt.Fprintf(w, "  Goalies:\n")
		fmt.Fprintf(w, "    %-22s %3s %6s %8s\n", "Name", "GP", "Shots", "Minutes")
		for _, g := range s.Goalies {
			name := g.Name
			if name == "" {
				name = "#" + strings.Join(g.Numbers, "/")
			}
			est := ""
			if g.Estimated {
				est = " *"
			}
			fmt.Fprintf(w, "    %-22s %3d %6d %8.2f%s\n", name, g.Games, g.Shots, g.Minutes, est)
		}
	}
	if len(s.Players) > 0 {
		fmt.Fprintf(w, "  Players:\n")
		fmt.Fprintf(w, "    %-22s %-6s %3s %3s %3s %4s\n", "Name", "No.", "GP", "G", "A", "Pts")
		for _, p := range s.Players {
			name := p.Name
			if name == "" {
				name = "(unknown)"
			}
			fmt.Fprintf(w, "    %-22s %-6s %3d %3d %3d %4d\n", name, strings.Join(p.Numbers, "/"), p.Games, p.Goals, p.Assists, p.Points())
		}
	}
}

// shotsStr renders "9 (4-3-2)".
func shotsStr(total int, per []int, known bool) string {
	if !known {
		return "n/a"
	}
	if len(per) == 0 {
		return strconv.Itoa(total)
	}
	parts := make([]string, len(per))
	for i, p := range per {
		parts[i] = strconv.Itoa(p)
	}
	return fmt.Sprintf("%d (%s)", total, strings.Join(parts, "-"))
}
