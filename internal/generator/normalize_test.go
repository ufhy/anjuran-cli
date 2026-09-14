package generator

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantName string
		wantDesc string
		wantOK   bool
	}{
		{"baris polos", "fitur-a", "fitur-a", "", true},
		{"spasi di tepi", "  fitur-a  ", "fitur-a", "", true},

		// Penanda branch aktif milik git; tanpa pembuangan ini kandidatnya
		// menjadi "* main", yang tidak mungkin disisipkan.
		{"penanda git", "* main", "main", "", true},
		{"penanda git berindentasi", "  * main", "main", "", true},
		{"branch biasa berindentasi", "  fitur-b", "fitur-b", "", true},

		// Keluaran berkolom: tab atau dua spasi memisahkan nama dari
		// keterangannya.
		{"kolom bertab", "asal\thttps://contoh.test (fetch)", "asal", "https://contoh.test (fetch)", true},
		{"kolom berspasi", "web     container berjalan", "web", "container berjalan", true},

		// Kandidat yang tidak bisa disisipkan apa adanya dibuang; perintah
		// yang rusak lebih buruk daripada daftar yang kosong.
		{"masih berspasi", "dua kata", "", "", false},
		{"baris kosong", "", "", "", false},
		{"hanya spasi", "   ", "", "", false},
		{"hanya tanda baca", "-------", "", "", false},
		{"garis tabel", "+---+---+", "", "", false},

		{"carriage return", "fitur-a\r", "fitur-a", "", true},
		{"angka saja", "8080", "8080", "", true},
		{"tanda hubung awal", "--force", "--force", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := normalize(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("normalize(%q) ok = %v, mau %v", tt.line, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, mau %q", got.Name, tt.wantName)
			}
			if got.Description != tt.wantDesc {
				t.Errorf("Description = %q, mau %q", got.Description, tt.wantDesc)
			}
		})
	}
}
