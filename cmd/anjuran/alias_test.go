package main

import "testing"

// Alias adalah cara sehari-hari orang memakai perintah panjang — oh-my-zsh
// sendiri memasang ratusan. Tanpa pemetaan ini, "gco" tidak menghasilkan apa
// pun karena anjuran mencari spec bernama gco.
func TestAliasDipetakanKeBentukSebenarnya(t *testing.T) {
	a := newAliasExpansion("gco fit", 7, "git checkout")
	if !a.active {
		t.Fatal("pemetaan seharusnya aktif")
	}
	if got := a.Line("gco fit"); got != "git checkout fit" {
		t.Errorf("Line = %q, mau %q", got, "git checkout fit")
	}
	if got := a.Cursor(7); got != 16 {
		t.Errorf("Cursor = %d, mau 16", got)
	}
}

// Hasilnya harus kembali ke baris ASLI. Menukar "gco" menjadi "git checkout"
// di layar mengubah apa yang pengguna ketik tanpa diminta, dan itu lebih
// mengganggu daripada tidak ada completion sama sekali.
func TestHasilDikembalikanKeBentukAlias(t *testing.T) {
	a := newAliasExpansion("gco fit", 7, "git checkout")
	line, cursor := a.Restore("git checkout fitur-a", 20)
	if line != "gco fitur-a" {
		t.Errorf("Line = %q, mau %q", line, "gco fitur-a")
	}
	if cursor != len("gco fitur-a") {
		t.Errorf("Cursor = %d, mau %d", cursor, len("gco fitur-a"))
	}
}

// Kursor yang masih berada di dalam nama alias berarti pengguna sedang
// mengetik nama aliasnya sendiri; memekarkannya di situ akan melengkapi
// perintah yang belum tentu ia maksud.
func TestKursorDiDalamNamaAliasTidakDimekarkan(t *testing.T) {
	for _, cursor := range []int{0, 1, 3} {
		a := newAliasExpansion("gco", cursor, "git checkout")
		if a.active {
			t.Errorf("kursor %d masih di dalam nama alias, seharusnya tidak dimekarkan", cursor)
		}
		if got := a.Line("gco"); got != "gco" {
			t.Errorf("Line = %q, mau utuh", got)
		}
	}
}

func TestTanpaAliasTidakMengubahApaPun(t *testing.T) {
	a := newAliasExpansion("git checkout fit", 16, "")
	if a.active {
		t.Error("tanpa pemekaran, pemetaan seharusnya tidak aktif")
	}
	if got := a.Line("git checkout fit"); got != "git checkout fit" {
		t.Errorf("Line = %q, mau utuh", got)
	}
	line, c := a.Restore("git checkout fitur-a", 20)
	if line != "git checkout fitur-a" || c != 20 {
		t.Errorf("Restore mengubah sesuatu: %q, %d", line, c)
	}
}

func TestAliasLebihPendekDariNamanya(t *testing.T) {
	// Alias bisa memekar menjadi teks yang lebih PENDEK, misalnya k='kubectl'
	// dibalik: "kubernetes" -> "k". Pergeserannya negatif.
	a := newAliasExpansion("panjangsekali get", 17, "k")
	if !a.active {
		t.Fatal("pemetaan seharusnya aktif")
	}
	if got := a.Line("panjangsekali get"); got != "k get" {
		t.Errorf("Line = %q, mau %q", got, "k get")
	}
	line, c := a.Restore("k get pods", 10)
	if line != "panjangsekali get pods" {
		t.Errorf("Line = %q, mau %q", line, "panjangsekali get pods")
	}
	if c != len("panjangsekali get pods") {
		t.Errorf("Cursor = %d, mau %d", c, len("panjangsekali get pods"))
	}
}

func TestBarisKosong(t *testing.T) {
	a := newAliasExpansion("", 0, "git checkout")
	if a.active {
		t.Error("baris kosong tidak punya kata pertama")
	}
}

// Perintah yang diawali separator tidak punya kata pertama yang bermakna.
func TestDiawaliSeparator(t *testing.T) {
	a := newAliasExpansion("| gco", 5, "git checkout")
	if a.active {
		t.Error("baris yang diawali separator seharusnya tidak dipetakan")
	}
}
