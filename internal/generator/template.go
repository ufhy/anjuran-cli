package generator

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Nama template bawaan yang dikenali.
const (
	TemplateFilepaths = "filepaths"
	TemplateFolders   = "folders"
	TemplateHistory   = "history"
	TemplateHelp      = "help" // belum didukung
)

// maxEntries membatasi jumlah entri yang dibaca dari satu direktori.
const maxEntries = 2000

// Files melengkapi nama berkas untuk sebuah prefix.
//
// Dikerjakan langsung, bukan dengan menjalankan ls: ini jalur yang paling
// sering dipakai, dan membaca direktori jauh lebih murah serta lebih aman
// daripada menumbuhkan proses baru setiap kali Tab ditekan.
//
// prefix adalah kata yang sedang diketik, boleh memuat bagian direktori
// seperti "internal/ge". Nilai kembaliannya sudah berisi bagian direktori itu,
// sehingga bisa langsung menggantikan kata tersebut.
func Files(prefix, workdir string, onlyDirs bool) []string {
	// "~" tanpa garis miring adalah rujukan ke direktori rumah, bukan nama
	// berkas yang diawali tilde.
	if prefix == "~" {
		return []string{"~/"}
	}

	dirPart, basePart := splitPrefix(prefix)

	// Tilde dipekarkan LEBIH DULU. Menggabungkannya dengan direktori kerja
	// sebelum itu menghasilkan "<cwd>/~", yang tidak pernah ada — dan "~/"
	// karena itu tidak menawarkan apa pun sama sekali.
	lookup := expandHome(dirPart)
	switch {
	case lookup == "":
		lookup = workdir
	case !filepath.IsAbs(lookup) && workdir != "":
		lookup = filepath.Join(workdir, lookup)
	}
	if lookup == "" {
		lookup = "."
	}

	entries, err := os.ReadDir(lookup)
	if err != nil {
		return nil
	}

	// Pencocokan mengabaikan besar-kecil huruf, seperti completion shell pada
	// umumnya. Mengetik "rea" lalu tidak mendapat README.md adalah kegagalan
	// yang membuat fiturnya terasa rusak, bukan teliti.
	lowerBase := strings.ToLower(basePart)

	var out []string
	for i, e := range entries {
		if i >= maxEntries {
			break
		}
		name := e.Name()

		isDir := e.IsDir()
		if !isDir && e.Type()&os.ModeSymlink != 0 {
			// Symlink ke direktori tetap layak diperlakukan sebagai direktori,
			// karena begitulah pengguna menelusurinya.
			if fi, err := os.Stat(filepath.Join(lookup, name)); err == nil {
				isDir = fi.IsDir()
			}
		}
		if onlyDirs && !isDir {
			continue
		}

		// Berkas tersembunyi hanya muncul bila memang sedang dicari.
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(basePart, ".") {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(name), lowerBase) {
			continue
		}

		if isDir {
			name += "/"
		}
		// Bagian direktori dikembalikan APA ADANYA, termasuk tilde-nya:
		// yang disisipkan harus tetap "~/berkas", bukan path rumah yang
		// sudah dipekarkan.
		out = append(out, dirPart+name)
	}

	sort.Strings(out)
	return out
}

// splitPrefix memisahkan bagian direktori dari kata yang sedang diketik.
// "internal/ge" menjadi ("internal/", "ge"), dan "ge" menjadi ("", "ge").
func splitPrefix(prefix string) (dir, base string) {
	i := strings.LastIndexAny(prefix, `/\`)
	if i < 0 {
		return "", prefix
	}
	return prefix[:i+1], prefix[i+1:]
}

// expandHome menerjemahkan awalan ~ menjadi direktori rumah.
func expandHome(p string) string {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if p == "~" {
		return home
	}
	return filepath.Join(home, p[2:])
}

// FromTemplates menjalankan seluruh template yang dikenali untuk sebuah prefix.
//
// command dibutuhkan oleh template "history", yang mencari argumen yang pernah
// dipakai bersama perintah itu — bukan baris perintahnya.
// Template yang belum didukung diabaikan diam-diam, karena ketiadaan kandidat
// jauh lebih baik daripada pesan kesalahan di tengah baris perintah.
func FromTemplates(templates []string, prefix, workdir, command string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range templates {
		var got []string
		switch t {
		case TemplateFilepaths:
			got = Files(prefix, workdir, false)
		case TemplateFolders:
			got = Files(prefix, workdir, true)
		case TemplateCommands:
			got = Commands(prefix)
		case TemplateHistory:
			got = HistoryArgs(command)
		case TemplateHosts:
			got = Hosts()
		case TemplateEnv:
			got = Env()
		}
		for _, c := range got {
			if !seen[c] {
				seen[c] = true
				out = append(out, c)
			}
		}
	}
	return out
}
