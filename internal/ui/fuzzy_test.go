package ui

import "testing"

func TestFuzzyMatchDasar(t *testing.T) {
	tests := []struct {
		name, query string
		wantMatch   bool
	}{
		{"checkout", "che", true},
		{"checkout", "ckt", true},   // subsequence
		{"checkout", "cho", true},   // subsequence dengan lompatan
		{"checkout", "xyz", false},  // huruf tidak ada
		{"checkout", "cheq", false}, // sebagian cocok saja tidak cukup
		{"commit", "", true},        // kueri kosong cocok dengan apa pun
		{"--force-with-lease", "fwl", true},
		{"CheckOut", "co", true},
	}

	for _, tt := range tests {
		got := FuzzyMatch(tt.name, tt.query)
		if (got != nil) != tt.wantMatch {
			t.Errorf("FuzzyMatch(%q, %q) cocok = %v, mau %v", tt.name, tt.query, got != nil, tt.wantMatch)
		}
	}
}

func TestAwalanMengalahkanSubsequence(t *testing.T) {
	prefix := FuzzyMatch("commit", "com")
	scattered := FuzzyMatch("checkout-mode", "com")
	if prefix == nil || scattered == nil {
		t.Fatal("keduanya seharusnya cocok")
	}
	if prefix.Score <= scattered.Score {
		t.Errorf("awalan (%d) harus menang atas subsequence tersebar (%d)", prefix.Score, scattered.Score)
	}
}

func TestBatasKataDapatBonus(t *testing.T) {
	boundary := FuzzyMatch("--force-with-lease", "fwl")
	middle := FuzzyMatch("affawalale", "fwl")
	if boundary == nil || middle == nil {
		t.Fatal("keduanya seharusnya cocok")
	}
	if boundary.Score <= middle.Score {
		t.Errorf("cocok di batas kata (%d) harus menang atas di tengah kata (%d)", boundary.Score, middle.Score)
	}
}

func TestNamaPendekMenangSaatSeri(t *testing.T) {
	short := FuzzyMatch("up", "up")
	long := FuzzyMatch("upgrade", "up")
	if short.Score <= long.Score {
		t.Errorf("nama pendek (%d) harus menang saat awalan sama (%d)", short.Score, long.Score)
	}
}

func TestPosisiUntukPenyorotan(t *testing.T) {
	m := FuzzyMatch("checkout", "cko")
	if m == nil {
		t.Fatal("seharusnya cocok")
	}
	want := []int{0, 4, 5} // c(0) ... k(4) o(5) pada "checkout"
	if len(m.Positions) != len(want) {
		t.Fatalf("Positions = %v, mau %v", m.Positions, want)
	}
	for i := range want {
		if m.Positions[i] != want[i] {
			t.Fatalf("Positions = %v, mau %v", m.Positions, want)
		}
	}
}
