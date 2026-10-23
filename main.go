// Command gamesheet-stats pulls a team's completed games from
// teams.gamesheet.app, downloads each official scoresheet PDF, and reports
// shots for/against, shots faced per goalie, and goals/assists per player,
// both as human-readable text and as CSV files.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wmarler/gamesheet-stats/internal/gamesheet"
	"github.com/wmarler/gamesheet-stats/internal/report"
	"github.com/wmarler/gamesheet-stats/internal/scoresheet"
	"github.com/wmarler/gamesheet-stats/internal/stats"
)

func main() {
	var (
		configPath = flag.String("config", "config.json", "path to config file (see config.example.json)")
		outDir     = flag.String("out", "output", "directory for CSV output")
		cacheDir   = flag.String("cache", "cache", "directory for cached tokens and scoresheet PDFs")
		refresh    = flag.Bool("refresh", false, "re-download scoresheet PDFs even if cached")
		listTeams  = flag.Bool("list-teams", false, "list the teams visible to this account and exit")
		teamFlag   = flag.String("team", "", "comma-separated team IDs to report on (overrides config)")
		noCSV      = flag.Bool("no-csv", false, "do not write CSV files")
		quiet      = flag.Bool("quiet", false, "suppress progress messages on stderr")
	)
	flag.Parse()

	if err := run(*configPath, *outDir, *cacheDir, *refresh, *listTeams, *teamFlag, *noCSV, *quiet); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(configPath, outDir, cacheDir string, refresh, listTeams bool, teamFlag string, noCSV, quiet bool) error {
	logf := func(format string, args ...any) {
		if !quiet {
			fmt.Fprintf(os.Stderr, format+"\n", args...)
		}
	}

	cfg, err := gamesheet.LoadConfig(configPath)
	if err != nil {
		return err
	}
	auth := gamesheet.NewAuthenticator(cfg, nil, cacheDir)
	client := gamesheet.NewClient(cfg, auth, nil)

	teams, err := client.Teams()
	if err != nil {
		return fmt.Errorf("listing teams: %w", err)
	}
	if listTeams {
		fmt.Printf("%-38s %-10s %-9s %s\n", "TEAM ID", "YEAR", "ARCHIVED", "NAME")
		for _, t := range teams {
			fmt.Printf("%-38s %-10s %-9v %s (%s %s, %s)\n", t.TeamID, t.StatsYear, t.IsArchived, t.TeamName, t.AgeCategory, t.Sport, t.Province)
		}
		return nil
	}

	// Decide which teams to report on.
	wanted := cfg.TeamIDs
	if teamFlag != "" {
		wanted = nil
		for _, id := range strings.Split(teamFlag, ",") {
			if id = strings.TrimSpace(id); id != "" {
				wanted = append(wanted, id)
			}
		}
	}
	byID := map[string]gamesheet.Team{}
	for _, t := range teams {
		byID[t.TeamID] = t
	}
	var selected []gamesheet.Team
	if len(wanted) == 0 {
		for _, t := range teams {
			if !t.IsArchived {
				selected = append(selected, t)
			}
		}
		logf("no team_ids configured; using %d non-archived team(s)", len(selected))
	} else {
		for _, id := range wanted {
			t, ok := byID[id]
			if !ok {
				return fmt.Errorf("team %s is not visible to this account (use -list-teams)", id)
			}
			selected = append(selected, t)
		}
	}

	// Collect games.
	var games []gamesheet.Game
	seen := map[string]bool{}
	for _, t := range selected {
		gs, err := client.Games(t.TeamID)
		if err != nil {
			return fmt.Errorf("fetching games for %s: %w", t.TeamName, err)
		}
		final, upcoming := 0, 0
		for _, g := range gs {
			if seen[g.ID] {
				continue
			}
			seen[g.ID] = true
			if g.IsFinal() {
				final++
			} else {
				upcoming++
			}
			games = append(games, g)
		}
		logf("%s: %d completed, %d upcoming/unscored game(s)", t.TeamName, final, upcoming)
	}
	sort.SliceStable(games, func(i, j int) bool {
		if games[i].EventDate != games[j].EventDate {
			return games[i].EventDate < games[j].EventDate
		}
		return games[i].EventTime < games[j].EventTime
	})

	// Download + parse scoresheets for completed games.
	pdfDir := filepath.Join(cacheDir, "pdfs")
	if err := os.MkdirAll(pdfDir, 0o755); err != nil {
		return err
	}
	var results []*stats.GameStats
	for _, g := range games {
		if !g.IsFinal() {
			continue
		}
		pdfPath := filepath.Join(pdfDir, g.ID+".pdf")
		data, err := os.ReadFile(pdfPath)
		if err != nil || refresh {
			logf("downloading scoresheet for game %s (%s %s)", g.ID, g.EventDate, g.EventTitle)
			data, err = client.ScoresheetPDF(g.ID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: game %s: %v (skipped)\n", g.ID, err)
				continue
			}
			_ = os.WriteFile(pdfPath, data, 0o644)
		}
		sheet, err := scoresheet.Parse(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: game %s: parsing scoresheet: %v (skipped)\n", g.ID, err)
			continue
		}
		if sheet.GameType == "" {
			if info, err := client.ScheduleGame(g.ID); err == nil {
				sheet.GameType = scoresheet.NormalizeAPIGameType(info.GameType)
			}
		}
		gs, err := stats.Build(g, sheet)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: game %s: %v (skipped)\n", g.ID, err)
			continue
		}
		results = append(results, gs)
	}

	report.WriteText(os.Stdout, results)

	if !noCSV {
		paths, err := report.WriteCSVs(outDir, results)
		if err != nil {
			return fmt.Errorf("writing CSV: %w", err)
		}
		logf("wrote %s", strings.Join(paths, ", "))
	}
	return nil
}
