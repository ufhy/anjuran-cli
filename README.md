# anjuran

Autocomplete **ala IDE** untuk shell, jalan di **Linux, macOS, dan Windows** —
termasuk lewat SSH, tmux, `docker exec`, dan `kubectl exec`.

Yang diambil dari Fig hanyalah **pengetahuan perintahnya** — 716 spec CLI
berlisensi MIT. Cara kerjanya diambil dari tempat lain: dari editor.

## Kenapa ada

[withfig/autocomplete](https://github.com/withfig/autocomplete) (kini bagian dari
kiro-cli) hanya jalan di macOS karena menggambar **window GUI transparan** di atas
kursor terminal lewat Accessibility API milik macOS. Pendekatan itu tidak bisa
dipindahkan: Wayland melarang positioning window absolut, dan overlay Win32 di atas
Windows Terminal bermasalah pada DPI serta compositing.

`anjuran` menggambar **di dalam terminal** memakai escape sequence ANSI, seperti fzf dan
zsh-autosuggestions. Konsekuensinya nol kode window-management per-OS — dan karena
UI-nya berupa byte, dropdown-nya melewati pipa SSH sama seperti output perintah biasa.

Model interaksinya juga bukan model Fig, melainkan model **IDE**: satu sesi
mengambil daftar kandidat sekali lalu menyaringnya di tempat sambil kamu
mengetik — satu proses per interaksi, bukan satu proses per huruf. Itu cara
kerja LSP, dan bersamanya ikut hal-hal yang sudah matang di editor: *trigger
characters*, `suggestSelection: recentlyUsedByPrefix`, dan *ghost text*. Masing-
masing dijelaskan di bawah.

Jadi pembagiannya: **datanya dari Fig, arsitekturnya dari editor.** Spec CLI
diambil ulang dari withfig/autocomplete yang berlisensi MIT.

## Rilis

`make snapshot` membangun rilis percobaan lengkap ke `dist/` tanpa
mempublikasikan apa pun; `git tag vX.Y.Z && git push --tags` menjalankan
rilis sungguhan lewat CI.

| Artefak | Tata letak spec |
|---|---|
| tar.gz, zip | `specs/` di samping binary |
| deb, rpm, apk | `/usr/share/anjuran/specs` |
| Homebrew cask | di dalam Caskroom, ditemukan lewat resolusi symlink |
| Scoop, winget | `specs/` di samping binary |

Paket rilis WAJIB memuat `specs/`. Tanpa itu binary-nya berjalan tetapi tidak
menawarkan apa pun, jadi alur rilis menggagalkan dirinya sendiri bila jumlah
spec yang terbangun kurang dari seribu.

## Spec

`make specs` mengunduh paket npm `@withfig/autocomplete` — yang sudah berisi spec
terkompilasi sebagai modul JS, jadi TypeScript tidak dibutuhkan sama sekali — lalu
menyaringnya menjadi skema anjuran.

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
1.635 dibuang** — jadi sebagian argumen dinamis tetap kosong. Contohnya
`kubectl get <TAB>` belum menawarkan tipe resource, karena spec Fig memasoknya
lewat closure JS.

Tidak ada generator yang `script`-nya berupa string shell. Tetapi bentuk argv
sendiri tidak mencegah eksekusi shell: 194 di antaranya berisi
`["bash","-c","<skrip>"]`. Itu ditangani oleh kebijakan generator, bukan oleh
bentuk datanya.

Spec disimpan ter-gzip dan dibaca langsung dari bentuk itu. Ini bukan penghematan
disk semata: rencana SSH mengharuskan spec ikut dikirim ke host remote, dan 7,7 MB
jauh berbeda dari 42 MB di sana.

### Tambalan bawaan

Spec bawaan lengkap pada bagian opsinya, tetapi banyak argumennya kosong: Fig
memasok isinya lewat closure JavaScript, dan 1.635 di antaranya tidak ikut
ter-transpile. Itulah sebabnya `ssh <TAB>` dan `cd <TAB>` tidak menawarkan apa
pun meski opsinya lengkap.

Direktori `extra/` menambal itu, dan isinya **digabung di atas** spec bawaan —
opsi, subcommand, dan deskripsi dari Fig tetap utuh:

| Perintah | Yang ditambal |
|---|---|
| `ssh`, `sftp`, `scp` | host dari `~/.ssh/config` dan `known_hosts` |
| `cd` | daftar direktori |
| `export`, `unset` | nama variabel lingkungan |
| `kubectl` | tipe resource, nama pod, namespace, context |
| `docker` | nama container dan image |
| `systemctl` | nama unit |
| `php` | opsi, `php artisan` beserta perintah proyekmu |
| `composer` | seluruh subcommand dari `composer list` |

Sebagian spec Fig praktis kosong karena isinya disusun saat runtime oleh
`generateSpec`, sebuah fungsi JavaScript. `php` misalnya hanya berisi nama dan
deskripsi. Ada 11 perintah seperti itu: `composer`, `php`, `rails`, `drush`,
`magento`, `kamal`, `task`, `z`, `mask`, `speedtest`, `create-video`.

### Saat spec tidak punya jawaban

Korpus Fig memuat 716 perintah. Sisanya — `gzip`, `awk`, `openssl`, perintah
internal perusahaan, skrip apa pun di PATH — tidak ada di sana, dan ratusan spec
yang ada pun hanya menyebutkan nama argumennya tanpa menyebut isinya dari mana.

Di kedua keadaan itu anjuran melengkapi **nama berkas**, sebagaimana shell mana pun
untuk perintah yang tidak dikenalnya. Diam total di situ salah: melengkapi path
adalah yang paling sering dibutuhkan.

### Syarat `whenFile`

Banyak subcommand hanya bermakna di dalam proyek tertentu. `php artisan` hanya
ada di proyek Laravel, dan menawarkannya di mana-mana membuat daftarnya
berbohong tentang apa yang sebenarnya bisa dijalankan.

```json
{ "name": "artisan", "whenFile": "artisan" }
```

Entri itu hanya muncul bila berkas atau direktori bernama itu ada di direktori
kerja. Berlaku untuk subcommand maupun suggestion, dan tidak menjalankan apa pun
— hanya satu pemeriksaan berkas.

Beberapa sumber dikerjakan anjuran sendiri tanpa menjalankan proses apa pun, dan
karena itu tidak tunduk pada kebijakan generator:

| Template | Sumber |
|---|---|
| `filepaths`, `folders` | isi direktori |
| `anjuran:hosts` | `~/.ssh/config` dan `~/.ssh/known_hosts` |
| `anjuran:env` | variabel lingkungan proses |
| `history` | argumen yang pernah dipakai bersama perintah itu |

### Spec sendiri

Spec dengan nama sama dari beberapa direktori **digabung**, bukan saling
menggantikan. Urutannya adalah urutan lapisan, dari yang paling menimpa:

```
~/.config/anjuran/specs        milikmu sendiri
./extra, <bin>/extra      tambalan bawaan
--specs, $ANJURAN_SPECS
./specs, <bin>/specs      spec hasil transpile
```

Jadi CLI internal cukup ditaruh di `~/.config/anjuran/specs/nama.json`. Berkas itu
tidak perlu lengkap — cukup memuat bagian yang ingin ditambal, karena sisanya
diambil dari lapisan di bawahnya.

Aturan penggabungannya sedikit dan sengaja bisa ditebak: subcommand dan opsi
dicocokkan berdasarkan nama lalu digabung ke dalam, argumen dicocokkan
berdasarkan urutan, dan sumber kandidat (`suggestions`, `generators`,
`template`) DIGANTI — karena yang di lapisan bawah memang itu yang ingin
diperbaiki.

## Status

Selesai enam tahap: jalan di **zsh, bash, fish, dan PowerShell**, dengan
**716 CLI** hasil impor dari paket spec Fig — git, docker, kubectl, terraform, aws, az, gcloud, npm, systemctl,
dan seterusnya.

| Tahap | Isi | Status |
|---|---|---|
| 1 | Parser + engine + spec buatan tangan | selesai |
| 2 | Renderer ANSI + integrasi zsh | selesai |
| 3 | Transpiler spec Fig → JSON, impor massal | selesai |
| 4 | Integrasi bash + fish | selesai |
| 5 | PowerShell / Windows Terminal | selesai |
| 6 | Rilis: brew, deb/rpm, scoop/winget | konfigurasi selesai, belum diterbitkan |

## Pasang

**Belum ada rilis yang diterbitkan.** Untuk sekarang pasang dari sumber —
lihat bagian di bawah. Perintah berikut baru akan bekerja setelah tag pertama
dibuat beserta kedua repo tap-nya:

```sh
brew install ufhy/tap/anjuran                 # macOS, Linux
scoop bucket add anjuran https://github.com/ufhy/scoop-bucket
scoop install anjuran                         # Windows
sudo dpkg -i anjuran_*_linux_amd64.deb        # Debian, Ubuntu
sudo rpm -i anjuran_*_linux_amd64.rpm         # Fedora, RHEL
```

Atau unduh arsip dari halaman rilis, letakkan `anjuran` di dalam PATH, dan biarkan
`specs/` bersebelahan dengannya. Tidak ada variabel lingkungan yang perlu
disetel: `anjuran` mencari spec relatif terhadap dirinya sendiri, termasuk saat
dipasang sebagai symlink oleh Homebrew.

### Dari sumber

```sh
make specs                              # unduh + transpile spec Fig (butuh node)
make build
sudo cp bin/anjuran /usr/local/bin/
mkdir -p ~/.config/anjuran && cp -r specs ~/.config/anjuran/
```

Lalu satu baris di berkas konfigurasi shell:

| Shell | Berkas | Baris |
|---|---|---|
| zsh | `~/.zshrc` | `eval "$(anjuran init zsh)"` |
| bash | `~/.bashrc` | `eval "$(anjuran init bash)"` |
| fish | `~/.config/fish/config.fish` | `anjuran init fish \| source` |
| PowerShell | `$PROFILE` | `anjuran init powershell \| Out-String \| Invoke-Expression` |

Skrip integrasinya disematkan di dalam binary, jadi tidak ada path repo yang
perlu diingat — dan memasang di host remote cukup berarti menyalin satu berkas.

### Dropdown yang muncul sendiri

Ketik `git` — dropdown muncul tanpa menekan apa pun, berisi seluruh subcommand
git beserta keterangannya. Tidak perlu spasi, tidak perlu Tab.

```
╭───────────────┬───────────────────────────────────────────────╮
│ ❯ add         │ Add file contents to the index                │
│   branch      │ List, create, or delete branches              │
│   checkout    │ Switch branches or restore working tree files │
╰───────────────┴───────────────────────────────────── 1/48 ────╯
```

Pemicunya bukan satu tombol khusus, melainkan titik-titik di mana ada sesuatu
yang layak ditawarkan: **mengetik nama perintah**, lalu **spasi**, **`/`**, dan
**`=`**. Ini mengikuti cara IDE bekerja — VS Code menyebutnya *trigger
characters*, dan di shell inilah padanannya.

Nyala secara bawaan. Matikan dengan:

```sh
ANJURAN_AUTO=0 eval "$(anjuran init zsh)"
```

Ambang panjang kata sebelum kotak dibuka diatur `ANJURAN_AUTO_MIN` (bawaan 2).
Satu huruf cocok dengan ratusan biner di PATH; daftar sepanjang itu tidak
menolong siapa pun.

### Nama perintah ikut dilengkapi

Di posisi perintah — awal baris, dan juga sesudah `|` atau `;` — yang ditawarkan
adalah biner di PATH:

```sh
kubec        →  kubectl, kubectl.docker
docker ps | gi  →  git, github, gitleaks
```

Begitu namanya cocok persis dengan perintah yang punya spec, isinya langsung
ditawarkan. Ditampilkan sebagai `add`, `commit` — bukan `git add` — sebagaimana
editor menampilkan anggota tanpa mengulang nama objeknya; yang disisipkan tetap
`git add`, karena yang diganti adalah kata perintahnya.

PATH dipindai sekali seumur proses, dan satu proses adalah satu interaksi.

### Satu sesi memegang seluruh interaksi

Begitu dropdown terbuka, anjuran yang membaca ketikan: menyaring di tempat,
menggemakan karakter, memindahkan pilihan. **Satu proses per interaksi, bukan
satu proses per huruf.**

Model ini diambil dari LSP: daftar kandidat diambil sekali, lalu disaring di
klien sambil pengguna mengetik — bukan dihitung ulang dari nol setiap ketikan.
Itu pula yang menghapus seluruh kelas bug sinkronisasi antara apa yang tergambar
dan apa yang ada di buffer.

Agar sesi tidak merampas apa pun dari zsh, tombol yang bukan urusan dropdown —
Ctrl-A, Home, panah kiri — **dikembalikan** ke antrean masukan zsh dan diproses
seperti tidak pernah lewat anjuran.

Widget yang sudah terpasang di spasi dan panah tetap dipanggil lebih dulu,
sehingga `magic-space` milik oh-my-zsh dan pencarian riwayat tetap bekerja.

### Saran dari riwayat

```sh
ANJURAN_GHOST=1 eval "$(anjuran init zsh)"
```

Teks abu-abu yang melanjutkan ketikanmu berdasarkan perintah yang pernah
dijalankan. Untuk perintah panjang yang diulang setiap hari — `kubectl logs -f`
dengan namespace dan selector — ini lebih sering menolong daripada dropdown.
Panah kanan atau `Ctrl-E` menerimanya.

Seluruhnya dikerjakan **di dalam zsh**, tanpa memanggil anjuran sama sekali: ia
diperbarui pada setiap ketikan, dan menumbuhkan proses di sana akan terasa
berat. Riwayat sudah ada di dalam shell; anjuran hanya menggambar dropdown.

Bayangan disembunyikan selama dropdown terbuka — dua saran sekaligus hanya
menambah kebisingan. Bila zsh-autosuggestions sudah terpasang, anjuran menyingkir
dan memberi tahu: keduanya memperebutkan `POSTDISPLAY` yang sama.

Warnanya diatur `ANJURAN_GHOST_STYLE`, memakai sintaks `region_highlight` zsh
(bawaannya `fg=8`).

### Mengingat pilihanmu

Kandidat yang pernah kamu pilih untuk sebuah awalan akan tersorot lebih dulu di
kali berikutnya — VS Code menyebutnya `suggestSelection: recentlyUsedByPrefix`.
Tanpa itu kamu menekan panah ke entri yang sama setiap hari.

Ingatannya sengaja **sempit**: kuncinya mencakup perintah DAN awalan yang
diketik. `git c` yang biasanya berakhir di `commit` tidak mengubah urutan
`docker c`.

Ia hanya **memindahkan**, tidak menambah: relevansi tetap yang memilih isi
daftarnya, dan kandidat yang tidak lagi cocok dengan yang kamu ketik tidak akan
dimunculkan kembali. Bobot sempat dicoba dan ditolak — bobot membuat urutannya
sulit dinalar, sementara memindahkan satu entri yang memang pernah kamu pilih
selalu bisa dijelaskan.

Tersimpan di `$ANJURAN_CACHE_DIR` atau direktori cache bawaan sistem; hapus
berkasnya untuk melupakan semuanya.

### Alias

Alias dikenali. `gco ` menawarkan nama branch karena zsh memberi tahu anjuran bahwa
`gco` berarti `git checkout`:

```
gco fit
╭───────────╮
│   fitur-a │
│   fitur-b │
╰─────── 2 ─╯
```

Perhitungannya memakai bentuk yang sudah dimekarkan, tetapi hasilnya
dikembalikan ke baris ASLI — yang tersisip adalah `gco fitur-a`, bukan
`git checkout fitur-a`. Menukar apa yang sudah kamu ketik lebih mengganggu
daripada tidak ada completion sama sekali.

Alias yang dicari adalah kata pertama dari segmen terakhir, sehingga
`docker ps | gst` memakai `gst`. Pemekarannya satu tingkat; alias yang menunjuk
alias lain tidak ditelusuri.

Kotak menghilang hanya bila tidak ada lagi yang cocok. Kandidat tunggal tetap
ditampilkan, tidak disisipkan sendiri — justru di situ kamu paling dekat dengan
jawabannya, dan mengubah baris perintah tanpa diminta membuat karakter yang
diketik sesudahnya mendarat di tempat yang salah. Hanya Tab yang berarti
"sisipkan yang itu".

Menempel satu baris panjang tidak membuka kotak sama sekali. Tempelan tiba
sekaligus dan dibaca zsh ke penyangganya sendiri; membuka sesi pada setiap spasi
di dalamnya berarti menunggu tombol yang sudah tidak ada di terminal — dan shell
terkunci. Selama masih ada ketikan yang menunggu dibaca, pemicunya diam.

Mengetik kata membuka kotak, tetapi biayanya **bukan** satu proses per huruf.
Begitu sesi terbuka ia memegang seluruh ketikan sampai kotaknya tertutup, jadi
huruf-huruf berikutnya disaring di dalam proses yang sama — paling banyak satu
proses per kata, dan sering satu per baris perintah.

Pembungkusnya memanggil widget yang sudah terpasang lebih dulu. zsh-autosuggestions
dan zsh-syntax-highlighting juga membungkus `self-insert`; memanggil
`zle .self-insert` begitu saja akan mematikan keduanya tanpa pesan apa pun.

Hanya zsh yang punya hook per-ketikan yang layak: bash
memerlukan `bind -x` pada setiap karakter, yang merusak bracketed paste dan
penanganan masukan readline; fish tidak punya hook itu; PSReadLine hanya
menyediakan pendaftaran per-tombol satu per satu.

Tab kini membuka dropdown. Tombol di dalamnya:

| Tombol | Aksi |
|---|---|
| ketik huruf | menyaring daftar secara langsung |
| Tab | sisipkan awalan terpanjang yang sama; bila tinggal satu kandidat, sisipkan kandidat itu |
| panah kanan | masuk ke dalam folder yang tersorot |
| Enter | sisipkan pilihan lalu tutup — tekan Enter lagi untuk menjalankan |
| Enter, tanpa ada yang tersorot | pakai baris apa adanya lalu tutup |
| panah bawah, Ctrl-N | turun |
| Shift-Tab, panah atas, Ctrl-P | naik |
| Page Down, Page Up | lompat satu layar |
| spasi | mengetik spasi, bukan memilih |
| Esc | tutup kotaknya, ketikan di dalamnya tetap dibawa |
| Ctrl-C | diteruskan ke shell, yang membatalkan barisnya |

Spasi **mengetik spasi**. Sempat dibuat menerima kandidat yang sedang tersorot,
dan itu mengejutkan: orang mengetik spasi untuk melanjutkan kalimat perintahnya,
bukan untuk memilih sesuatu yang kebetulan berada di baris teratas. Menerima
harus selalu tindakan yang disengaja — Tab atau Enter.

Esc **tidak membuang ketikan**. Yang dibatalkan hanya sarannya; karakter yang
sudah kamu ketik di dalam sesi tetap milikmu.

Tombol yang tidak dikenali sesi — Ctrl-A, Home, panah kiri — dikembalikan ke
zsh, bukan ditelan.

### Folder

Baris folder yang tersorot menunjukkan kedua tombolnya:

```
╭────────────────────╮
│ ❯ berkas-lain/ → ⏎ │      → masuk ke dalamnya
│   berkas/          │      ⏎ berhenti, pakai path ini
│   cache/           │
│   proyek/          │
╰────────────── 1/4 ─╯
```

Menelusuri ke dalam folder membuka isinya **tanpa memilihkan apa pun**. Itulah
cara berhenti: Enter di situ berarti "cukup, pakai path ini", sedangkan Tab atau
panah kanan turun satu tingkat lagi. Sebelumnya isinya dibuka dengan anak pertama
tersorot — sehingga Enter, satu-satunya cara berhenti, justru turun lagi; dan
bila anaknya tunggal ia disisipkan lalu ditelusuri lagi, sampai dasar.

Nama berspasi ditangani tanpa perlu kamu mengutipnya lebih dulu:

```sh
cd folder de<TAB>        →  cd 'folder dengan spasi/'
```

Shell sudah memecah `folder de` menjadi dua kata sebelum `anjuran` melihatnya,
jadi prefix-nya disatukan kembali melintasi spasi — tetapi hanya bila gabungan
itu benar-benar cocok dengan sesuatu di disk. `ls berkas catatan.txt` tetap dua
argumen, dan `gzip folder -d` tetap sebuah opsi.

Tombol pemicunya bisa diganti bila Tab ingin dibiarkan milik shell:

```sh
ANJURAN_KEY='^ '   eval "$(anjuran init zsh)"     # zsh:  Ctrl-Spasi
ANJURAN_KEY='\C-@' eval "$(anjuran init bash)"    # bash: Ctrl-Spasi
set -gx ANJURAN_KEY \cspace                  # fish: Ctrl-Spasi
$env:ANJURAN_KEY = 'Ctrl+Spacebar'           # PowerShell
```

### Saat sebuah perintah tidak punya spec

`anjuran` mengembalikan tombolnya ke shell, jadi Tab tidak pernah terasa mati:

| Shell | Yang terjadi |
|---|---|
| zsh | `expand-or-complete` bawaan, utuh |
| fish | `commandline -f complete` bawaan, utuh |
| PowerShell | `MenuComplete` bawaan, utuh |
| bash | melengkapi nama berkas saja |

Bash memang lebih terbatas, dan itu batasan readline, bukan pilihan desain:
`bind -x` mengambil alih tombolnya sepenuhnya dan tidak menyediakan cara
memanggil kembali fungsi yang didaftarkan `complete`. Yang ditiru karena itu
hanya perilaku bawaan readline, yaitu melengkapi path. Siapa pun yang lebih
membutuhkan bash-completion daripada anjuran sebaiknya memindahkan pemicunya ke
tombol lain lewat `ANJURAN_KEY`.

Bash 4.0 ke atas dibutuhkan, karena `READLINE_LINE` dan `READLINE_POINT` baru
ada sejak versi itu. Bash 3.2 bawaan macOS tidak didukung; pasang lewat
`brew install bash`.

## Coba tanpa memasang

```sh
make build
./bin/anjuran complete --line "git commit --"
./bin/anjuran complete --line "kubectl get pods -o " --json
```

`--cursor` menerima offset byte bila kursor tidak berada di akhir baris:

```sh
./bin/anjuran complete --line "git checkout" --cursor 7   # melengkapi "che"
```

## Arsitektur

```
cmd/anjuran/            entry point CLI: perintah complete dan widget
internal/spec/     model skema spec Fig + loader JSON
internal/parser/   tokenizer sadar-kutip + resolusi posisi kursor
internal/engine/   penelusuran pohon spec → daftar kandidat
internal/ui/       pencocokan fuzzy, renderer diff, loop interaktif
internal/generator/ eksekusi generator, kebijakannya, cache, template berkas
internal/remote/   pemasangan ke host lain lewat SSH
internal/tty/      mode raw, ukuran layar, penguraian tombol
internal/shellinit/ skrip integrasi shell, disematkan ke binary
tools/transpile/   pengubah spec Fig menjadi skema anjuran
specs/             DIHASILKAN oleh `make specs`, tidak masuk git
internal/testdata/ spec buatan tangan sebagai fixture pengujian
```

Hanya `internal/tty/open_*.go` yang bergantung pada sistem operasi, dan isinya
sebatas cara membuka perangkat terminal. Penguraian tombol, penyusunan dropdown,
dan seluruh logika lain sama persis di ketiga platform.

`internal/engine` hampir murni: masukannya `(string, int)`, keluarannya struct.
Satu-satunya sentuhannya ke dunia luar adalah pemeriksaan berkas untuk
`whenFile`, yang bisa diganti saat pengujian. Tidak
menyentuh terminal, tidak punya state global, dan tidak mengeksekusi apa pun.
Seluruh perilakunya teruji tanpa PTY, dan nantinya bisa dipakai ulang oleh editor
atau language server.

## Dua syarat non-fungsional

Keduanya mahal bila di-retrofit, jadi dipegang sejak tahap 1:

1. **Renderer harus diff-based.** Di SSH dengan RTT 200ms, repaint layar penuh
   setiap keystroke terasa lag. Renderer hanya mengirim baris yang berubah:

   | Kejadian | Byte terkirim |
   |---|---|
   | dropdown pertama kali digambar | ~1.600 |
   | pindah pilihan satu baris | ~430 |
   | render dengan isi identik | 4 |

   Garis bingkai tidak pernah berubah antar penekanan tombol, jadi ia hanya
   dikirim sekali untuk seluruh sesi meski dropdown digambar berkali-kali.

   Mode degradasi `ANJURAN_SIMPLE=1` mematikan warna dan sorotan, dan menyala
   otomatis untuk `TERM` bernilai `dumb`, `vt100`, `vt102`, atau `ansi`.

   Waktu yang dirasakan saat dropdown muncul sendiri:

   | | |
   |---|---|
   | spasi → gambar kotak | 3,1 ms |
   | huruf berikutnya → menyaring | 3,1 ms |
   | hapus kotak | 3,1 ms |

   Waktu hitung engine per ketikan:

   | | |
   |---|---|
   | spec sudah di cache | 1,9 µs |
   | lewat `loadSpec` (aws s3) | 2,3 µs |
   | muat dingin, buka gzip + urai | 1,2 ms |

   Yang dirasakan pengguna lebih besar dari itu, karena `anjuran` adalah proses
   baru setiap kali Tab ditekan:

   | | |
   |---|---|
   | menyalakan proses saja | 3,2 ms |
   | completion statis | 6,6 ms |
   | dengan generator, cache panas | 6,8 ms |

   Jadi biaya terbesarnya adalah menyalakan proses, bukan menghitung. Daemon
   yang tetap hidup akan memangkas 3,2 ms itu; tidak dibangun karena
   pengukurannya menunjukkan tidak perlu.

   Generator yang GAGAL ikut di-cache sebentar. Tanpa itu, mengetik di
   direktori yang bukan repo git menjalankan `git branch` yang gagal
   berulang-ulang — 32 ms, bukan 7 ms. Generator yang gagal justru yang paling
   mahal, karena biayanya dibayar penuh tanpa pernah menghasilkan apa pun.
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
| zsh | `$CURSOR` | rune |
| fish | `commandline -C` | rune |
| bash | `$READLINE_POINT` | **byte** |
| PowerShell | `GetBufferState` | **UTF-16 code unit** |

Tiga satuan berbeda, jadi `anjuran widget` menerima `--cursor-unit rune\|byte\|utf16`.
Salah satuan tidak terlihat sama sekali pada baris ASCII. Ia baru muncul saat
baris memuat karakter non-ASCII, dan tiap satuan berpisah pada titik berbeda:

| Karakter | byte | rune | utf16 |
|---|---|---|---|
| `a` | 1 | 1 | 1 |
| `é` | 2 | 1 | 1 |
| `日` | 3 | 1 | 1 |
| `🚀` | 4 | 1 | **2** |

Diuji di keempat shell dengan `café`, `日本語`, dan emoji.

## Generator

Sebagian argumen tidak bisa diketahui dari berkas spec: nama branch, nama pod,
nama container. Untuk itu spec menyimpan **generator**, yaitu perintah yang
dijalankan untuk menghasilkan kandidat.

```
git checkout <TAB>   ->  git branch      ->  nama branch di repo ini
git push <TAB>       ->  git remote      ->  nama remote sungguhan
cat <TAB>            ->  dibaca langsung ->  nama berkas, tanpa proses baru
```

### Kebijakan

Menekan Tab akan **menjalankan perintah** — bukan membaca berkas, melainkan
menumbuhkan proses di direktori kerjamu dengan hak aksesmu. Karena itu
kebijakannya ketat secara bawaan:

| Aturan | Alasan |
|---|---|
| Hanya menjalankan perintah yang sedang kamu ketik | `git checkout` boleh memanggil `git`, dan hanya `git` |
| Interpreter ditolak | `bash`, `sh`, `python`, `node`, `sudo`, `env`, `xargs` — argumennya adalah kode, bukan data |
| Mati saat berjalan sebagai root | Di server, satu Tab yang salah jauh lebih mahal |
| Batas waktu keras 1,2 detik | Generator lambat tidak boleh menahan tombol |
| Tidak pernah lewat shell | argv dieksekusi langsung; tidak ada string yang diurai sebagai perintah |
| Seluruh keturunan proses ikut dimatikan | Cucu proses yang tertinggal akan menumpuk |

Larangan interpreter bukan kehati-hatian berlebih. **194 generator di paket
spec Fig berbentuk `["bash","-c","<skrip>"]`** — bentuk argv sendiri tidak
mencegah eksekusi shell, hanya memindahkan pintunya. Ada pula generator yang
memanggil `curl`, yang berarti permintaan jaringan sebagai efek samping
mengetik.

Akibatnya sebagian generator memang tidak akan pernah berjalan. Itu pilihan
sadar: Tab yang tidak menawarkan apa-apa jauh lebih murah daripada Tab yang
menjalankan sesuatu yang tidak kamu minta.

### Yang tepercaya adalah asal spec-nya, bukan binernya

Larangan interpreter sebetulnya menyasar **argumen yang isinya kode**, dan nama
biner hanyalah perkiraan kasar untuk itu. `php artisan list` bukan kode;
`php -r <apa pun>` jelas kode. Yang membedakan keduanya bukan binernya,
melainkan siapa yang menulis argv-nya.

Karena itu spec yang ditulis tangan dan ditinjau — `extra/` bawaan dan
`~/.config/anjuran/specs` milikmu — boleh melewati larangan itu. Korpus hasil
transpile tidak pernah: 1.472 berkas yang tidak pernah dibaca seorang pun, dan
194 di antaranya memang berisi `["bash","-c","<skrip>"]`.

Penanda tepercaya itu **tidak bisa diisi dari JSON**. Kalau bisa, berkas spec
mana pun tinggal menyatakan dirinya tepercaya dan seluruh kebijakannya runtuh;
nilainya dipasang oleh pemuat spec berdasarkan direktori asal berkasnya.
Kelonggaran ini juga tidak menembus aturan lain — biner yang berbeda dari
perintah yang kamu ketik tetap ditolak, dan berjalan sebagai root tetap
mematikan semuanya.

```sh
ANJURAN_NO_GENERATORS=1               # matikan seluruhnya
ANJURAN_GENERATOR_ALLOW=tmux,kubectl  # izinkan biner tambahan
ANJURAN_GENERATOR_ALLOW_ROOT=1        # izinkan berjalan sebagai root
ANJURAN_GENERATOR_TIMEOUT=800ms       # ubah batas waktu
```

`anjuran complete --line "..."` menjalankan jalur yang sama persis dengan Tab, dan
mencetak alasan setiap penolakan ke stderr — jadi apa yang akan dijalankan anjuran
bisa diperiksa sebelum dipasang.

### Keluaran mentah

`postProcess` milik Fig berupa closure JavaScript dan tidak ikut ter-transpile,
sehingga keluaran perintah dinormalkan dengan aturan yang sedikit dan
eksplisit: tab atau dua spasi memisahkan nama dari keterangannya, penanda `* `
milik git dibuang, dan kandidat yang masih mengandung spasi dibuang seluruhnya
— teks yang tidak bisa disisipkan apa adanya menghasilkan perintah yang rusak,
bukan sekadar tampilan yang jelek.

### Cache

Keluaran generator disimpan di disk, bukan di memori: `anjuran` adalah proses baru
setiap kali Tab ditekan, jadi cache dalam memori tidak akan pernah terpakai
sekali pun. Kuncinya mencakup direktori kerja, karena `git branch` menjawab
berbeda di setiap repo.

## Pengujian

Selain uji unit per paket, ada **uji cakupan** di `internal/generator` yang
menjalankan JALUR PENUH — engine, spec sungguhan, tambalan, template, dan
generator — lalu memeriksa bahwa perintah yang benar-benar dipakai orang
menghasilkan sesuatu:

| Uji | Yang dijaga |
|---|---|
| `TestCakupanPerintahUmum` | ~75 perintah sehari-hari tidak diam |
| `TestCakupanOpsi` | daftar opsi tersedia |
| `TestCakupanBentukPath` | `~/`, `./`, `../`, `/abs/`, subdirektori |
| `TestCakupanArgumenDinamis` | argumen dari tambalan buatan tangan |
| `TestPerintahTanpaSpecTetapMelengkapiBerkas` | perintah asing tetap berguna |
| `TestSyaratWhenFile` | entri bersyarat muncul di tempat yang tepat |
| `TestSpecKosongYangDiketahui` | daftar kekosongan yang tersisa, agar terlihat |

Di atasnya ada **penyapuan korpus**: menjalankan SELURUH spec terpasang — 716
perintah lintas banyak posisi kursor — lalu memeriksa sifat yang harus benar
untuk semuanya. Contoh yang dipilih tangan hanya menemukan yang sudah
terpikirkan.

| Sifat yang dijaga | Yang pernah ditemukannya |
|---|---|
| kandidat terurai utuh sebagai kata shell | 131 kandidat berspasi, kutip menggantung di `mysql` |
| tidak ada kandidat kembar | 43 entri ganda |
| rentang penggantian memuat kursor | — |
| setiap spec bisa diurai | 5 spec mati total karena `requiresSeparator` |
| keterangan tanpa karakter kendali | byte NUL di spec `ag` |
| setiap baris kotak sama lebar DI LAYAR | CJK dan emoji mematahkan bingkai |
| cabang `loadSpec` menghasilkan sesuatu | — |
| masukan acak tidak menjatuhkan engine | — |

### Uji UX di shell sungguhan

`make ux` menjalankan skenario yang benar-benar diketik orang di dalam **zsh
asli dengan konfigurasi shell pengguna**, lalu memeriksa apa yang TERLIHAT di
layar — bukan jawaban API. Escape sequence yang keluar diputar ulang menjadi
kisi teks, lengkap dengan penggulungan layar dan pemulihan posisi kursor.

Uji Go memeriksa jawaban engine; ini memeriksa pengalamannya: apakah kotaknya
muncul, apakah isinya benar, apakah ia hilang saat seharusnya hilang.

Dijalankan dengan konfigurasi asli dan BUKAN zsh kosong — zsh kosong
menyembunyikan seluruh kelas bug, karena di sana spasi terpasang ke
`self-insert` sementara oh-my-zsh memetakannya ke `magic-space`.

```
make ux                 jalankan semuanya
make ux SKENARIO=alias  jalankan yang namanya memuat "alias"
```

Uji ini lahir dari kegagalan berulang: bug yang dilaporkan pengguna berkali-kali
lolos dari pengujian sebelumnya, karena skenarionya dipilih sendiri dan selalu
yang sudah diketahui bekerja. Daftar perintahnya diambil dari yang dipakai
sehari-hari, bukan dari yang mudah lulus — dan begitu ditulis, ia langsung
menemukan enam perintah yang diam.

## Windows

PowerShell 5.1 ke atas, dengan PSReadLine yang sudah menjadi bawaannya. Windows
Terminal mendukung VT penuh sejak 2019, jadi renderer yang sama langsung
berlaku; pada conhost lama `anjuran` menyalakan `ENABLE_VIRTUAL_TERMINAL_PROCESSING`
sendiri, dan bila gagal hanya warnanya yang hilang. `cmd.exe` tidak didukung.

Yang bergantung pada Windows hanyalah `internal/tty/open_windows.go`, sebatas
membuka `CONIN$` dan `CONOUT$`. Berkas itu **belum pernah dijalankan di Windows
sungguhan** dari sini — pengembangannya di macOS. Karena itu CI menjalankan
seluruh uji di runner `windows-latest`, dan skrip PowerShell-nya diverifikasi di
PowerShell 7.6 dengan PSReadLine 2.4 lewat PTY.

## SSH

Tidak ada penanganan khusus. Engine dijalankan **di sisi remote**, persis seperti
Tab bawaan shell: laptopmu mengirim satu byte, host remote yang menghitung, hasilnya
kembali sebagai teks. Ini juga membuat generator benar — `kubectl get pods`
mengembalikan pod di cluster server itu, bukan konteks kubectl laptopmu.

Syaratnya sama dengan syarat Tab supaya pintar di sana: satu binary statis di host,
satu baris di rc file.

```sh
anjuran bootstrap deploy@web-01
```

Perintah itu mendeteksi platform host, mengalirkan binary dan spec lewat koneksi
SSH yang sama, lalu memeriksa hasilnya dengan benar-benar menjalankan berkas yang
dikirim. Menyalin bukan berarti bisa menjalankan: home yang dipasang `noexec`
baru ketahuan di langkah itu.

```
  host      : deploy@web-01 (linux/amd64)
  sumber    : anjuran_1.0.0_linux_amd64.tar.gz
  binary    : ~/.local/bin/anjuran
  spec      : ~/.local/share/anjuran/specs
  terkirim  : 6.6 MB
  terpasang : anjuran 1.0.0
```

Tata letaknya mengikuti XDG, sehingga penemuan spec berjalan tanpa variabel
lingkungan: `~/.local/bin/anjuran` mencari `../share/anjuran/specs` dan menemukannya.

### Yang TIDAK dilakukan perintah ini

**Bukan pembungkus `ssh`.** anjuran tidak pernah menyisip di antara kamu dan
koneksimu, tidak mengubah `~/.ssh/config`, dan tidak pernah berjalan otomatis
saat kamu menyambung ke suatu host. Perintah ini dijalankan sekali, dengan
sadar, lalu selesai.

Sebelum mengirim apa pun, rencananya ditampilkan dan persetujuan diminta.
Di luar terminal interaktif jawabannya selalu tidak — sebuah skrip tidak boleh
mendapat izin hanya karena tidak ada yang menjawab. Untuk pemakaian terskrip
ada dua jalan yang meninggalkan jejak: `--yes`, atau mendaftarkan host di
`~/.config/anjuran/hosts` — persetujuan yang bisa ditinjau dan disimpan di kendali
versi, bukan jawaban di layar yang menguap.

```
# ~/.config/anjuran/hosts
web-01
web-*.internal
deploy@bastion
```

### Platform berbeda

Tanpa `--from`, satu-satunya sumber yang tersedia adalah binary yang sedang
berjalan, jadi platformnya harus sama. Untuk host yang berbeda, sebutkan
sumbernya:

```sh
make snapshot                                  # atau unduh arsip rilis
anjuran bootstrap --from dist web-01
```

Mesin lokal yang mengunduh, bukan host — jadi ini tetap bekerja untuk server
produksi yang tidak punya akses internet keluar.

Argumen setelah nama host diteruskan apa adanya ke `ssh`, sehingga `ProxyJump`,
bastion, dan opsi lain berlaku seperti biasa:

```sh
anjuran bootstrap web-01 -J bastion -i ~/.ssh/deploy
```

Perintah `ssh` sistem yang dipakai, bukan pustaka SSH — supaya seluruh isi
`~/.ssh/config` berlaku apa adanya, termasuk Match block dan kunci perangkat
keras. Koneksinya dibagi lewat `ControlMaster`: bootstrap memanggil host
beberapa kali, dan tanpa itu host ber-MFA akan meminta sentuhan kunci keamanan
berkali-kali untuk satu perintah.
