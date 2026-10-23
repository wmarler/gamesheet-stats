package scoresheet

import "testing"

func TestGameTypeFromStream(t *testing.T) {
	const panel = `
[<524547554c415220534541534f4e> 0] TJ
0 0 4 4 re
B
[<45584849424954494f4e> 0] TJ
0 0 4 4 re
S
[<54> 40 <4f55524e414d454e54> 0] TJ
0 0 4 4 re
S
[<504c41> 100 <59> 85 <4f4646> 0] TJ
0 0 4 4 re
S
[<504c41594f46462047414d45> 0] TJ
0 0 172 24 re
S
`
	if got := gameTypeFromStream(panel); got != "League" {
		t.Errorf("checked regular season: got %q", got)
	}
	tournament := `
[<524547554c415220534541534f4e> 0] TJ
0 0 4 4 re
S
[<45584849424954494f4e> 0] TJ
0 0 4 4 re
S
[<54> 40 <4f55524e414d454e54> 0] TJ
0 0 4 4 re
B
[<504c41> 100 <59> 85 <4f4646> 0] TJ
0 0 4 4 re
S
`
	if got := gameTypeFromStream(tournament); got != "Tournament" {
		t.Errorf("checked tournament: got %q", got)
	}
	if got := gameTypeFromStream("[<504c41594f4646> 0] TJ\n0 0 4 4 re\nB\n[<504c41594f4646> 0] TJ\n0 0 4 4 re\nB\n"); got != "" {
		t.Errorf("two boxes checked should be unknown, got %q", got)
	}
}

func TestNormalizeAPIGameType(t *testing.T) {
	cases := map[string]string{
		"regular_season": "League",
		"tournament":     "Tournament",
		"exhibition":     "Exhibition",
		"playoff":        "Playoff",
		"scrimmage":      "",
		"":               "",
	}
	for in, want := range cases {
		if got := NormalizeAPIGameType(in); got != want {
			t.Errorf("NormalizeAPIGameType(%q) = %q, want %q", in, got, want)
		}
	}
}
