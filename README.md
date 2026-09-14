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

## Spec

`make specs` mengunduh paket npm `@withfig/autocomplete` — yang sudah berisi spec
terkompilasi sebagai modul JS, jadi TypeScript tidak dibutuhkan sama sekali — lalu
menyaringnya menjadi skema uf.

| | |
|---|---|
| spec | 1.472 berkas, 716 perintah tingkat atas |
| subcommand | 52.255 |
| opsi | 282.836 |
| saran statis | 27.192 |
| ukuran | 42 MB sebagai JSON, 7,7 MB ter-gzip |

Yang dibuang saat transpile adalah segala sesuatu yang membutuhkan mesin
JavaScript saat runtime: generator berbentuk fungsi dan `custom`, `postProcess`,
`generateSpec`, dan `parserDirectives`. Dari 5.073 generator, **3.438 terbawa dan
1.635 dibuang** — jadi sebagian argumen dinamis akan kosong sampai layer generator
dibangun. Contohnya `kubectl get <TAB>` belum menawarkan tipe resource, karena
spec Fig memasoknya lewat closure JS.

Tidak ada satu pun generator yang berbentuk string shell, sehingga tidak ada
perintah yang perlu dilewatkan ke `sh -c`.

Spec disimpan ter-gzip dan dibaca langsung dari bentuk itu. Ini bukan penghematan
disk semata: rencana SSH mengharuskan spec ikut dikirim ke host remote, dan 7,7 MB
jauh berbeda dari 42 MB di sana.

### Spec sendiri

`uf` mencari spec secara berurutan, dan direktori pertama yang memuat berkasnya
menang:

```
--specs <dir>            (boleh beberapa, dipisah titik dua)
$UF_SPECS
~/.config/uf/specs
./specs
<dir binary>/specs
```

Jadi CLI internal cukup ditaruh di `~/.config/uf/specs/nama.json`, dan spec bawaan
yang dianggap kurang tepat bisa ditimpa tanpa menyunting direktori yang
dihasilkan mesin.

## Status

Tahap 4 dari 6: sudah bisa dipakai di **zsh, bash, dan fish**, dengan **716 CLI**
hasil impor dari paket spec Fig — git, docker, kubectl, terraform, aws, az, gcloud, npm, systemctl,
dan seterusnya.

| Tahap | Isi | Status |
|---|---|---|
| 1 | Parser + engine + spec buatan tangan | selesai |
| 2 | Renderer ANSI + integrasi zsh | selesai |
| 3 | Transpiler spec Fig → JSON, impor massal | selesai |
| 4 | Integrasi bash + fish | selesai |
| 5 | PowerShell / Windows Terminal | belum |
| 6 | Rilis: brew, deb/rpm, scoop/winget | belum |

## Pasang

```sh
make specs                              # unduh + transpile spec Fig (butuh node)
make build
sudo cp bin/uf /usr/local/bin/          # atau taruh di mana pun dalam PATH
mkdir -p ~/.config/uf && cp -r specs ~/.config/uf/
```

Lalu satu baris di berkas konfigurasi shell:

| Shell | Berkas | Baris |
|---|---|---|
| zsh | `~/.zshrc` | `eval "$(uf init zsh)"` |
| bash | `~/.bashrc` | `eval "$(uf init bash)"` |
| fish | `~/.config/fish/config.fish` | `uf init fish \| source` |

Skrip integrasinya disematkan di dalam binary, jadi tidak ada path repo yang
perlu diingat — dan memasang di host remote cukup berarti menyalin satu berkas.

Tab kini membuka dropdown. Tombol di dalamnya:

| Tombol | Aksi |
|---|---|
| ketik huruf | menyaring daftar secara langsung |
| Tab, panah bawah, Ctrl-N | turun |
| Shift-Tab, panah atas, Ctrl-P | naik |
| Enter | sisipkan pilihan |
| Spasi | sisipkan lalu tutup, siap mengetik argumen berikutnya |
| Esc, Ctrl-C | batal, baris dibiarkan apa adanya |

Tombol pemicunya bisa diganti bila Tab ingin dibiarkan milik shell:

```sh
UF_KEY='^ '   eval "$(uf init zsh)"     # zsh:  Ctrl-Spasi
UF_KEY='\C-@' eval "$(uf init bash)"    # bash: Ctrl-Spasi
set -gx UF_KEY \cspace                  # fish: Ctrl-Spasi
```

### Saat sebuah perintah tidak punya spec

`uf` mengembalikan tombolnya ke shell, jadi Tab tidak pernah terasa mati:

| Shell | Yang terjadi |
|---|---|
| zsh | `expand-or-complete` bawaan, utuh |
| fish | `commandline -f complete` bawaan, utuh |
| bash | melengkapi nama berkas saja |

Bash memang lebih terbatas, dan itu batasan readline, bukan pilihan desain:
`bind -x` mengambil alih tombolnya sepenuhnya dan tidak menyediakan cara
memanggil kembali fungsi yang didaftarkan `complete`. Yang ditiru karena itu
hanya perilaku bawaan readline, yaitu melengkapi path. Siapa pun yang lebih
membutuhkan bash-completion daripada uf sebaiknya memindahkan pemicunya ke
tombol lain lewat `UF_KEY`.

Bash 4.0 ke atas dibutuhkan, karena `READLINE_LINE` dan `READLINE_POINT` baru
ada sejak versi itu. Bash 3.2 bawaan macOS tidak didukung; pasang lewat
`brew install bash`.

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
internal/shellinit/ skrip integrasi shell, disematkan ke binary
tools/transpile/   pengubah spec Fig menjadi skema uf
specs/             DIHASILKAN oleh `make specs`, tidak masuk git
internal/testdata/ spec buatan tangan sebagai fixture pengujian
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

   Waktu hitung per ketikan, jauh di bawah anggaran 16 ms:

   | | |
   |---|---|
   | spec sudah di cache | 1,9 µs |
   | lewat `loadSpec` (aws s3) | 2,3 µs |
   | muat dingin, buka gzip + urai | 1,2 ms |
2. **Generator butuh policy layer.** Generator mengeksekusi perintah sebagai efek
   samping mengetik. Karena itu `internal/engine` sengaja hanya *melaporkan*
   generator yang relevan tanpa menjalankannya — eksekusinya ditaruh di satu
   tempat terpisah yang memegang allowlist, timeout keras, dan default mati saat
   UID 0.

Skema generator juga sudah berbentuk argv (`["git","branch"]`), bukan string shell,
sehingga tidak ada jalur injeksi lewat berkas spec.

## Satuan posisi kursor

Setiap shell melaporkan posisi kursor dengan satuan berbeda, dan ini bukan
detail yang bisa diabaikan:

| Shell | Variabel | Satuan |
|---|---|---|
| zsh | `$CURSOR` | karakter |
| fish | `commandline -C` | karakter |
| bash | `$READLINE_POINT` | **byte** |

Karena itu `uf widget` menerima `--cursor-unit rune\|byte`, dan hanya skrip bash
yang memintanya dalam byte. Salah satuan tidak terlihat sama sekali pada baris
ASCII, dan baru muncul sebagai sisipan di posisi yang salah begitu ada huruf
beraksen atau CJK di baris itu.

## SSH

Tidak ada penanganan khusus. Engine dijalankan **di sisi remote**, persis seperti
Tab bawaan shell: laptopmu mengirim satu byte, host remote yang menghitung, hasilnya
kembali sebagai teks. Ini juga membuat generator benar — `kubectl get pods`
mengembalikan pod di cluster server itu, bukan konteks kubectl laptopmu.

Syaratnya sama dengan syarat Tab supaya pintar di sana: satu binary statis di host,
satu baris di rc file.
