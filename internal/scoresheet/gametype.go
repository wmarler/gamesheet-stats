package scoresheet

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"io"
	"regexp"
	"strings"
)

// gameTypeLabels are the four checkboxes in the scoresheet GAME TYPE panel,
// in the order they are drawn, and the names we report for them.
var gameTypeLabels = []struct{ text, name string }{
	{"REGULAR SEASON", "League"},
	{"EXHIBITION", "Exhibition"},
	{"TOURNAMENT", "Tournament"},
	{"PLAYOFF", "Playoff"},
}

var reHexString = regexp.MustCompile(`<([0-9A-Fa-f]+)>`)

// NormalizeAPIGameType maps a schedule-game "game_type" value onto the same
// labels the scoresheet checkboxes use. Unknown values return "".
func NormalizeAPIGameType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "regular_season", "regular", "league":
		return "League"
	case "tournament":
		return "Tournament"
	case "exhibition":
		return "Exhibition"
	case "playoff", "playoffs":
		return "Playoff"
	default:
		return ""
	}
}

// gameType reads the GAME TYPE checkboxes. A checked box is a 4x4 rectangle
// painted with the "B" operator (fill); an empty box is only stroked ("S").
// The box is not text, so it does not show up in the text extractor. Returns
// "" when the panel cannot be read or more than one box is checked.
func gameType(data []byte) string {
	for _, stream := range contentStreams(data) {
		if t := gameTypeFromStream(stream); t != "" {
			return t
		}
	}
	return ""
}

func gameTypeFromStream(content string) string {
	var checked []string
	var label string
	sawBox := false
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasSuffix(line, "TJ"):
			label = gameTypeLabel(line)
			sawBox = false
		case line == "0 0 4 4 re":
			sawBox = true
		case sawBox && (line == "B" || line == "b" || line == "f" || line == "F"):
			if label != "" {
				checked = append(checked, label)
			}
			label, sawBox = "", false
		case sawBox && (line == "S" || line == "s"):
			label, sawBox = "", false
		default:
			if line != "" && !strings.HasPrefix(line, "%") {
				// A different drawing command means this was not the checkbox
				// immediately following the label.
				if !strings.HasSuffix(line, "re") {
					sawBox = false
				}
			}
		}
	}
	if len(checked) == 1 {
		return checked[0]
	}
	return ""
}

// gameTypeLabel decodes one TJ line and returns the game-type name it is,
// or "" for any other text. "PLAYOFF GAME" (a different field) is not the
// PLAYOFF checkbox.
func gameTypeLabel(tjLine string) string {
	var b strings.Builder
	for _, m := range reHexString.FindAllStringSubmatch(tjLine, -1) {
		raw, err := hex.DecodeString(m[1])
		if err != nil {
			continue
		}
		b.Write(raw)
	}
	text := b.String()
	if text == "" {
		return ""
	}
	for _, l := range gameTypeLabels {
		if l.text == "PLAYOFF" {
			if strings.Contains(text, "PLAYOFF") && !strings.Contains(text, "GAME") {
				return l.name
			}
			continue
		}
		if strings.Contains(text, l.text) {
			return l.name
		}
	}
	return ""
}

// contentStreams returns the decompressed content streams of a PDF.
func contentStreams(data []byte) []string {
	var out []string
	rest := data
	for {
		i := bytes.Index(rest, []byte("stream"))
		if i < 0 {
			break
		}
		rest = rest[i+len("stream"):]
		if bytes.HasPrefix(rest, []byte("\r\n")) {
			rest = rest[2:]
		} else if bytes.HasPrefix(rest, []byte("\n")) {
			rest = rest[1:]
		}
		j := bytes.Index(rest, []byte("endstream"))
		if j < 0 {
			break
		}
		chunk := bytes.TrimRight(rest[:j], "\r\n")
		rest = rest[j+len("endstream"):]
		r, err := zlib.NewReader(bytes.NewReader(chunk))
		if err != nil {
			continue
		}
		decoded, err := io.ReadAll(r)
		r.Close()
		if err != nil || !bytes.Contains(decoded, []byte("4 4 re")) {
			continue
		}
		out = append(out, string(decoded))
	}
	return out
}
