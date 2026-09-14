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

Tahap 2 dari 6: sudah bisa dipakai di zsh. Tekan Tab, dropdown muncul, ketik untuk
menyaring, panah untuk memilih, Enter untuk menyisipkan.

| Tahap | Isi | Status |
|---|---|---|
| 1 | Parser + engine + spec buatan tangan | selesai |
| 2 | Renderer ANSI + integrasi zsh | selesai |
| 3 | Transpiler spec Fig TS → JSON | belum |
| 4 | Integrasi bash + fish | belum |
| 5 | PowerShell / Windows Terminal | belum |
| 6 | Rilis: brew, deb/rpm, scoop/winget | belum |

## Pasang di zsh

```sh
make build
sudo cp bin/uf /usr/local/bin/          # atau taruh di mana pun dalam PATH
mkdir -p ~/.config/uf && cp -r specs ~/.config/uf/
echo 'source /path/ke/uf-cli/shell/uf.zsh' >> ~/.zshrc
```

Tab kini membuka dropdown. Tombol di dalamnya:

| Tombol | Aksi |
|---|---|
| ketik huruf | menyaring daftar secara langsung |
| Tab, panah bawah, Ctrl-N | turun |
| Shift-Tab, panah atas, Ctrl-P | naik |
| Enter | sisipkan pilihan |
| Spasi | sisipkan lalu tutup, siap mengetik argumen berikutnya |
| Esc, Ctrl-C | batal, baris dibiarkan apa adanya |

Tombol pemicunya bisa diganti bila Tab ingin dibiarkan milik zsh:

```sh
UF_KEY='^ ' source /path/ke/uf-cli/shell/uf.zsh   # Ctrl-Spasi
```

Bila sebuah perintah tidak punya spec, `uf` menyerahkannya kembali ke
`expand-or-complete` bawaan zsh — jadi Tab tidak pernah terasa mati.

## Coba tanpa memasang

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
cmd/uf/            entry point CLI: perintah complete dan widget
internal/spec/     model skema spec Fig + loader JSON
internal/parser/   tokenizer sadar-kutip + resolusi posisi kursor
internal/engine/   penelusuran pohon spec → daftar kandidat
internal/ui/       pencocokan fuzzy, renderer diff, loop interaktif
internal/tty/      mode raw, ukuran layar, penguraian tombol
shell/             integrasi per-shell
specs/             spec CLI dalam JSON
```

Hanya `internal/tty/open_*.go` yang bergantung pada sistem operasi, dan isinya
sebatas cara membuka perangkat terminal. Penguraian tombol, penyusunan dropdown,
dan seluruh logika lain sama persis di ketiga platform.

`internal/engine` murni: masukannya `(string, int)`, keluarannya struct. Tidak
menyentuh terminal, tidak punya state global, dan tidak mengeksekusi apa pun.
Seluruh perilakunya teruji tanpa PTY, dan nantinya bisa dipakai ulang oleh editor
atau language server.

## Dua syarat non-fungsional

Keduanya mahal bila di-retrofit, jadi dipegang sejak tahap 1:

1. **Renderer harus diff-based.** Di SSH dengan RTT 200ms, repaint layar penuh
   setiap keystroke terasa lag. Renderer hanya mengirim baris yang berubah:

   | Kejadian | Byte terkirim |
   |---|---|
   | dropdown pertama kali digambar | 880 |
   | pindah pilihan satu baris | 167 |
   | render dengan isi identik | 4 |

   Mode degradasi `UF_SIMPLE=1` mematikan warna dan sorotan, dan menyala
   otomatis untuk `TERM` bernilai `dumb`, `vt100`, `vt102`, atau `ansi`.
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
