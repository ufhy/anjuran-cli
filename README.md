# uf

Autocomplete gaya Fig untuk shell, jalan di **Linux, macOS, dan Windows** —
termasuk lewat SSH, tmux, `docker exec`, dan `kubectl exec`.

## Kenapa ada

[withfig/autocomplete](https://github.com/withfig/autocomplete) (kini bagian dari
kiro-cli) hanya jalan di macOS karena menggambar **window GUI transparan** di atas
kursor terminal lewat Accessibility API milik macOS. Pendekatan itu tidak bisa
dipindahkan: Wayland melarang positioning window absolut, dan overlay Win32 di atas
Windows Terminal bermasalah pada DPI serta compositing.

`uf` menggambar **di dalam terminal** memakai escape sequence ANSI, seperti fzf dan
zsh-autosuggestions. Konsekuensinya nol kode window-management per-OS — dan karena
UI-nya berupa byte, dropdown-nya melewati pipa SSH sama seperti output perintah biasa.

Spec CLI diambil ulang dari withfig/autocomplete yang berlisensi MIT.

## Status

Tahap 1 dari 6: parser, engine, dan spec. **Belum ada integrasi shell** — saat ini
`uf` masih berupa perintah yang dipanggil manual.

| Tahap | Isi | Status |
|---|---|---|
| 1 | Parser + engine + spec buatan tangan | selesai |
| 2 | Renderer ANSI + integrasi zsh | belum |
| 3 | Transpiler spec Fig TS → JSON | belum |
| 4 | Integrasi bash + fish | belum |
| 5 | PowerShell / Windows Terminal | belum |
| 6 | Rilis: brew, deb/rpm, scoop/winget | belum |

## Coba

```sh
make build
./bin/uf complete --line "git commit --"
./bin/uf complete --line "kubectl get pods -o " --json
```

`--cursor` menerima offset byte bila kursor tidak berada di akhir baris:

```sh
./bin/uf complete --line "git checkout" --cursor 7   # melengkapi "che"
```

## Arsitektur

```
cmd/uf/            entry point CLI
internal/spec/     model skema spec Fig + loader JSON
internal/parser/   tokenizer sadar-kutip + resolusi posisi kursor
internal/engine/   penelusuran pohon spec → daftar kandidat
specs/             spec CLI dalam JSON
```

`internal/engine` murni: masukannya `(string, int)`, keluarannya struct. Tidak
menyentuh terminal, tidak punya state global, dan tidak mengeksekusi apa pun.
Seluruh perilakunya teruji tanpa PTY, dan nantinya bisa dipakai ulang oleh editor
atau language server.

## Dua syarat non-fungsional

Keduanya mahal bila di-retrofit, jadi dipegang sejak tahap 1:

1. **Renderer harus diff-based.** Di SSH dengan RTT 200ms, repaint layar penuh
   setiap keystroke terasa lag. Plus mode degradasi `UF_SIMPLE=1` untuk
   `TERM=dumb`, vt100, dan konsol perangkat jaringan.
2. **Generator butuh policy layer.** Generator mengeksekusi perintah sebagai efek
   samping mengetik. Karena itu `internal/engine` sengaja hanya *melaporkan*
   generator yang relevan tanpa menjalankannya — eksekusinya ditaruh di satu
   tempat terpisah yang memegang allowlist, timeout keras, dan default mati saat
   UID 0.

Skema generator juga sudah berbentuk argv (`["git","branch"]`), bukan string shell,
sehingga tidak ada jalur injeksi lewat berkas spec.

## SSH

Tidak ada penanganan khusus. Engine dijalankan **di sisi remote**, persis seperti
Tab bawaan shell: laptopmu mengirim satu byte, host remote yang menghitung, hasilnya
kembali sebagai teks. Ini juga membuat generator benar — `kubectl get pods`
mengembalikan pod di cluster server itu, bukan konteks kubectl laptopmu.

Syaratnya sama dengan syarat Tab supaya pintar di sana: satu binary statis di host,
satu baris di rc file.
