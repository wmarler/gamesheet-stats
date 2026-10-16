package scoresheet

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/ledongthuc/pdf"
)

// run is a contiguous string drawn at one position on the page.
type run struct {
	X, Y float64
	S    string
}

// extractRuns returns the text runs for every page of the PDF, in page order.
//
// The scoresheet uses the standard (non-embedded) Helvetica font without a
// Widths array, so the PDF library cannot advance X within a string: every
// glyph of a string reports the same X (give or take kerning). The library
// does, however, emit a "\n" pseudo-glyph at the end of each drawn string, so
// we reconstruct strings by concatenating glyphs in content-stream order up to
// each "\n", and use the first glyph's position as the string's position.
func extractRuns(data []byte) ([][]run, float64, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, 0, fmt.Errorf("open pdf: %w", err)
	}
	var pages [][]run
	var width float64
	for pn := 1; pn <= r.NumPage(); pn++ {
		page := r.Page(pn)
		if page.V.IsNull() {
			continue
		}
		if width == 0 {
			if mb := page.V.Key("MediaBox"); mb.Len() == 4 {
				width = mb.Index(2).Float64() - mb.Index(0).Float64()
			}
		}
		var runs []run
		var cur *run
		flush := func() {
			if cur != nil {
				cur.S = strings.TrimSpace(cur.S)
				if cur.S != "" {
					runs = append(runs, *cur)
				}
				cur = nil
			}
		}
		for _, t := range page.Content().Text {
			if t.S == "\n" {
				flush()
				continue
			}
			if cur == nil {
				cur = &run{X: t.X, Y: t.Y}
			}
			cur.S += t.S
		}
		flush()
		pages = append(pages, runs)
	}
	if len(pages) == 0 {
		return nil, 0, fmt.Errorf("pdf has no pages")
	}
	return pages, width, nil
}

// groupRows clusters runs into rows by Y (descending, i.e. top of page first)
// and sorts each row left-to-right. Runs whose Y differ by less than tol are
// considered the same row.
func groupRows(runs []run, tol float64) [][]run {
	sorted := append([]run(nil), runs...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Y > sorted[j].Y })
	var rows [][]run
	for _, rn := range sorted {
		if n := len(rows); n > 0 && rows[n-1][0].Y-rn.Y < tol {
			rows[n-1] = append(rows[n-1], rn)
			continue
		}
		rows = append(rows, []run{rn})
	}
	for _, row := range rows {
		sort.SliceStable(row, func(i, j int) bool { return row[i].X < row[j].X })
	}
	return rows
}

// nearest returns the index of the anchor X closest to x, or -1 if none is
// within maxDist.
func nearest(anchors []float64, x, maxDist float64) int {
	best, bestD := -1, maxDist
	for i, a := range anchors {
		if d := abs(a - x); d <= bestD {
			best, bestD = i, d
		}
	}
	return best
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
