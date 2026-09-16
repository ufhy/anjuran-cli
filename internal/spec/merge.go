package spec

// Penggabungan spec.
//
// Spec bawaan hasil transpile lengkap pada bagian opsinya, tetapi argumennya
// banyak yang kosong: Fig memasok isinya lewat closure JavaScript yang tidak
// bisa dibawa. Tambalan karena itu harus MENAMBAH, bukan menggantikan — spec
// kubectl bawaan berisi ribuan opsi, dan menukarnya dengan berkas tambalan
// kecil akan menghilangkan semuanya.
//
// Aturannya sengaja sedikit supaya bisa ditebak tanpa membaca kode:
//
//   - Subcommand dan opsi dicocokkan berdasarkan nama, lalu digabung ke dalam.
//     Yang belum ada ditambahkan.
//   - Argumen dicocokkan berdasarkan URUTAN. Tambalan yang menyebut suggestion,
//     generator, atau template akan MENGGANTI milik dasar untuk argumen itu —
//     karena yang di dasar memang biasanya kosong atau tidak bisa dijalankan.
//   - Field teks hanya ditimpa bila tambalan mengisinya.

// merge menerapkan overlay di atas base, lalu mengembalikan hasilnya.
// base tidak diubah.
func merge(base, overlay *Subcommand) *Subcommand {
	if base == nil {
		return overlay
	}
	if overlay == nil {
		return base
	}

	out := *base
	if len(overlay.Name) > 0 {
		out.Name = overlay.Name
	}
	if overlay.Description != "" {
		out.Description = overlay.Description
	}
	if overlay.DisplayName != "" {
		out.DisplayName = overlay.DisplayName
	}
	if overlay.InsertValue != "" {
		out.InsertValue = overlay.InsertValue
	}
	if overlay.Priority != 0 {
		out.Priority = overlay.Priority
	}
	if overlay.LoadSpec != "" {
		out.LoadSpec = overlay.LoadSpec
	}
	if overlay.WhenFile != "" {
		out.WhenFile = overlay.WhenFile
	}
	out.Hidden = out.Hidden || overlay.Hidden
	out.Deprecated = out.Deprecated || overlay.Deprecated
	out.IsDangerous = out.IsDangerous || overlay.IsDangerous

	out.Subcommands = mergeSubcommands(base.Subcommands, overlay.Subcommands)
	out.Options = mergeOptions(base.Options, overlay.Options)
	out.Args = mergeArgs(base.Args, overlay.Args)
	return &out
}

func mergeSubcommands(base, overlay []Subcommand) []Subcommand {
	if len(overlay) == 0 {
		return base
	}
	out := append([]Subcommand(nil), base...)

	for i := range overlay {
		o := &overlay[i]
		if j := indexByName(out, o.Name); j >= 0 {
			out[j] = *merge(&out[j], o)
			continue
		}
		out = append(out, *o)
	}
	return out
}

func mergeOptions(base, overlay []Option) []Option {
	if len(overlay) == 0 {
		return base
	}
	out := append([]Option(nil), base...)

	for i := range overlay {
		o := overlay[i]
		j := indexOptionByName(out, o.Name)
		if j < 0 {
			out = append(out, o)
			continue
		}

		m := out[j]
		if o.Description != "" {
			m.Description = o.Description
		}
		if o.InsertValue != "" {
			m.InsertValue = o.InsertValue
		}
		if o.Priority != 0 {
			m.Priority = o.Priority
		}
		m.IsPersistent = m.IsPersistent || o.IsPersistent
		m.IsRepeatable = m.IsRepeatable || o.IsRepeatable
		if o.RequiresSeparator.Required {
			m.RequiresSeparator = o.RequiresSeparator
		}
		m.IsDangerous = m.IsDangerous || o.IsDangerous
		m.Hidden = m.Hidden || o.Hidden
		m.Args = mergeArgs(m.Args, o.Args)
		out[j] = m
	}
	return out
}

// mergeArgs mencocokkan argumen berdasarkan urutan, karena argumen posisional
// memang tidak punya nama yang bisa diandalkan.
func mergeArgs(base, overlay []Arg) []Arg {
	if len(overlay) == 0 {
		return base
	}
	out := append([]Arg(nil), base...)

	for i := range overlay {
		o := overlay[i]
		if i >= len(out) {
			out = append(out, o)
			continue
		}

		m := out[i]
		if o.Name != "" {
			m.Name = o.Name
		}
		if o.Description != "" {
			m.Description = o.Description
		}
		// Sumber kandidat diganti SELURUHNYA begitu tambalan menyebutkan
		// salah satunya: yang ada di dasar justru yang ingin diperbaiki.
		//
		// Mengganti per bidang menyisakan sumber yang lama. "bun run" tetap
		// membawa generator `bash -c ... cat package.json` milik spec Fig
		// walaupun tambalannya sudah memberi template pengganti — dan
		// generator itu lalu ditolak kebijakan pada setiap penekanan tombol,
		// menghasilkan pekerjaan dan pesan yang tidak ada gunanya.
		if len(o.Suggestions) > 0 || len(o.Generators) > 0 || len(o.Template) > 0 {
			m.Suggestions, m.Generators, m.Template = o.Suggestions, o.Generators, o.Template
		}
		m.IsOptional = m.IsOptional || o.IsOptional
		m.IsVariadic = m.IsVariadic || o.IsVariadic
		out[i] = m
	}
	return out
}

func indexByName(list []Subcommand, name Names) int {
	for i := range list {
		for _, n := range name {
			if list[i].Name.Has(n) {
				return i
			}
		}
	}
	return -1
}

func indexOptionByName(list []Option, name Names) int {
	for i := range list {
		for _, n := range name {
			if list[i].Name.Has(n) {
				return i
			}
		}
	}
	return -1
}

// markTrusted menandai seluruh generator di dalam sebuah spec sebagai
// tepercaya, dipanggil setelah berkasnya dibaca dari direktori tepercaya.
func markTrusted(sc *Subcommand) {
	if sc == nil {
		return
	}
	for i := range sc.Args {
		markArgTrusted(&sc.Args[i])
	}
	for i := range sc.Options {
		for j := range sc.Options[i].Args {
			markArgTrusted(&sc.Options[i].Args[j])
		}
	}
	for i := range sc.Subcommands {
		markTrusted(&sc.Subcommands[i])
	}
}

func markArgTrusted(a *Arg) {
	for i := range a.Generators {
		a.Generators[i].Trusted = true
	}
}
