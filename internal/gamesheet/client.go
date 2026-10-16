package gamesheet

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to the GameSheet Teams API gateway.
type Client struct {
	cfg    *Config
	auth   *Authenticator
	http   *http.Client
	tokens *Tokens
}

func NewClient(cfg *Config, auth *Authenticator, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &Client{cfg: cfg, auth: auth, http: httpClient}
}

// Team is one entry from GET /api/teams.
type Team struct {
	TeamID           string `json:"teamId"`
	TeamName         string `json:"teamName"`
	AgeCategory      string `json:"ageCategory"`
	Province         string `json:"province"`
	Sport            string `json:"sport"`
	Role             string `json:"role"`
	IsArchived       bool   `json:"isArchived"`
	StatsYear        string `json:"statsYear"`
	HasActiveSeasons bool   `json:"hasActiveSeasons"`
}

// Game is one "game" event from GET /api/calendar.
type Game struct {
	ID            string `json:"id"`
	EventTitle    string `json:"eventTitle"`
	EventDate     string `json:"eventDate"` // YYYY-MM-DD
	EventTime     string `json:"eventTime"` // HH:MM:SS
	EventLocation string `json:"eventLocation"`
	Timezone      string `json:"timezone"`
	EventData     struct {
		GameNumber   string `json:"gameNumber"`
		GameStatus   string `json:"gameStatus"` // "final", "scheduled", ...
		HomeTeam     string `json:"homeTeam"`
		VisitorTeam  string `json:"visitorTeam"`
		HomeGoals    *int   `json:"homeGoals"`
		VisitorGoals *int   `json:"visitorGoals"`
		IsHome       bool   `json:"isHome"`
		Result       string `json:"result"` // "Win", "Loss", "Tie"
		SeasonID     int    `json:"seasonId"`
	} `json:"eventData"`

	// TeamID is the prototeam the calendar was fetched for (filled in by us).
	TeamID string `json:"-"`
}

// IsFinal reports whether the game has been completed and scored.
func (g Game) IsFinal() bool { return strings.EqualFold(g.EventData.GameStatus, "final") }

// Opponent returns the opposing team's name.
func (g Game) Opponent() string {
	if g.EventData.IsHome {
		return g.EventData.VisitorTeam
	}
	return g.EventData.HomeTeam
}

// MyTeamName returns our team's name as it appears in this game's season.
func (g Game) MyTeamName() string {
	if g.EventData.IsHome {
		return g.EventData.HomeTeam
	}
	return g.EventData.VisitorTeam
}

// Teams lists the teams visible to the signed-in user.
func (c *Client) Teams() ([]Team, error) {
	var out struct {
		Success bool   `json:"success"`
		Teams   []Team `json:"teams"`
		Error   string `json:"error"`
	}
	if err := c.getJSON("/api/teams", nil, &out); err != nil {
		return nil, err
	}
	if !out.Success {
		return nil, fmt.Errorf("teams: %s", out.Error)
	}
	return out.Teams, nil
}

// Games returns every game on the team's calendar (past and future).
func (c *Client) Games(teamID string) ([]Game, error) {
	q := url.Values{}
	q.Set("teamId", teamID)
	q.Set("month", "all")
	q.Set("eventType", "game")
	var out struct {
		Success bool   `json:"success"`
		Data    []Game `json:"data"`
		Error   string `json:"error"`
	}
	if err := c.getJSON("/api/calendar", q, &out); err != nil {
		return nil, err
	}
	if !out.Success {
		return nil, fmt.Errorf("calendar: %s", out.Error)
	}
	for i := range out.Data {
		out.Data[i].TeamID = teamID
	}
	return out.Data, nil
}

// ScoresheetPDF downloads the official scoresheet PDF for a game.
func (c *Client) ScoresheetPDF(gameID string) ([]byte, error) {
	resp, err := c.do(http.MethodGet, "/api/scoresheets/games/"+url.PathEscape(gameID)+"/download", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "pdf") && !strings.HasPrefix(string(body), "%PDF") {
		return nil, fmt.Errorf("scoresheet %s: unexpected content type %q", gameID, ct)
	}
	return body, nil
}

func (c *Client) getJSON(path string, q url.Values, out any) error {
	resp, err := c.do(http.MethodGet, path, q)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%s: bad JSON: %w", path, err)
	}
	return nil
}

// do performs an authenticated request, transparently re-authenticating once
// on a 401.
func (c *Client) do(method, path string, q url.Values) (*http.Response, error) {
	for attempt := 0; attempt < 2; attempt++ {
		if c.tokens == nil {
			t, err := c.auth.Tokens()
			if err != nil {
				return nil, err
			}
			c.tokens = t
		}
		u := c.cfg.APIBase + path
		if len(q) > 0 {
			u += "?" + q.Encode()
		}
		req, err := http.NewRequest(method, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.tokens.Access)
		req.Header.Set("Accept", "application/json, application/pdf, */*")
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", method, path, err)
		}
		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			resp.Body.Close()
			// Force a fresh login on the next loop iteration.
			c.tokens = nil
			if t, err := c.auth.Login(); err == nil {
				c.tokens = t
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			return nil, fmt.Errorf("%s %s: HTTP %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(body)))
		}
		return resp, nil
	}
	return nil, fmt.Errorf("%s %s: authentication failed", method, path)
}
