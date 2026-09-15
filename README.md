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

## Rilis

`make snapshot` membangun rilis percobaan lengkap ke `dist/` tanpa
mempublikasikan apa pun; `git tag vX.Y.Z && git push --tags` menjalankan
rilis sungguhan lewat CI.

| Artefak | Tata letak spec |
|---|---|
| tar.gz, zip | `specs/` di samping binary |
| deb, rpm, apk | `/usr/share/uf/specs` |
| Homebrew cask | di dalam Caskroom, ditemukan lewat resolusi symlink |
| Scoop, winget | `specs/` di samping binary |

Paket rilis WAJIB memuat `specs/`. Tanpa itu binary-nya berjalan tetapi tidak
menawarkan apa pun, jadi alur rilis menggagalkan dirinya sendiri bila jumlah
spec yang terbangun kurang dari seribu.

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
| 6 | Rilis: brew, deb/rpm, scoop/winget | selesai |

## Pasang

```sh
brew install uf-cli/tap/uf              # macOS, Linux
scoop bucket add uf-cli https://github.com/uf-cli/scoop-bucket
scoop install uf                        # Windows
sudo dpkg -i uf_*_linux_amd64.deb       # Debian, Ubuntu
sudo rpm -i uf_*_linux_amd64.rpm        # Fedora, RHEL
```

Atau unduh arsip dari halaman rilis, letakkan `uf` di dalam PATH, dan biarkan
`specs/` bersebelahan dengannya. Tidak ada variabel lingkungan yang perlu
disetel: `uf` mencari spec relatif terhadap dirinya sendiri, termasuk saat
dipasang sebagai symlink oleh Homebrew.

### Dari sumber

```sh
make specs                              # unduh + transpile spec Fig (butuh node)
make build
sudo cp bin/uf /usr/local/bin/
mkdir -p ~/.config/uf && cp -r specs ~/.config/uf/
```

Lalu satu baris di berkas konfigurasi shell:

| Shell | Berkas | Baris |
|---|---|---|
| zsh | `~/.zshrc` | `eval "$(uf init zsh)"` |
| bash | `~/.bashrc` | `eval "$(uf init bash)"` |
| fish | `~/.config/fish/config.fish` | `uf init fish \| source` |
| PowerShell | `$PROFILE` | `uf init powershell \| Out-String \| Invoke-Expression` |

Skrip integrasinya disematkan di dalam binary, jadi tidak ada path repo yang
perlu diingat — dan memasang di host remote cukup berarti menyalin satu berkas.

### Dropdown yang muncul sendiri

```sh
UF_AUTO=1 eval "$(uf init zsh)"
```

Ketik `git` lalu **spasi** — dropdown muncul tanpa menekan apa pun. Huruf
berikutnya menyaringnya.

Kotak yang muncul sendiri sengaja tidak menyorot baris mana pun: kamu masih
mengetik, dan Enter di situ menjalankan perintah. Tekan **panah** untuk masuk
ke mode memilih — panah bawah mulai dari baris pertama, panah atas dari yang
terakhir. Tab juga bisa. Saat kotak tertutup, panah tetap menjadi riwayat
perintah seperti biasa.

Widget yang sudah terpasang di spasi dan panah tetap dipanggil lebih dulu,
sehingga `magic-space` milik oh-my-zsh dan pencarian riwayat tetap bekerja.

Kotak menghilang hanya pada dua keadaan: tidak ada yang cocok, atau yang tersisa
tinggal satu dan teksnya sudah diketik penuh. Kandidat tunggal yang belum
selesai diketik tetap ditampilkan — justru di situ kamu paling dekat dengan
jawabannya.

Pemicunya spasi, bukan setiap huruf. Sebelum sebuah kata selesai, isi dropdown
hanya akan berganti-ganti mengikuti huruf yang belum tentu selesai — dan
biayanya akan dibayar pada tombol yang paling sering ditekan. Satu penggambaran
memakan 3,1 ms; di spasi itu tidak terasa, di setiap huruf akan terasa.

Bawaannya mati. Hanya zsh yang punya hook per-ketikan yang layak: bash
memerlukan `bind -x` pada setiap karakter, yang merusak bracketed paste dan
penanganan masukan readline; fish tidak punya hook itu; PSReadLine hanya
menyediakan pendaftaran per-tombol satu per satu.

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
$env:UF_KEY = 'Ctrl+Spacebar'           # PowerShell
```

### Saat sebuah perintah tidak punya spec

`uf` mengembalikan tombolnya ke shell, jadi Tab tidak pernah terasa mati:

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
internal/generator/ eksekusi generator, kebijakannya, cache, template berkas
internal/remote/   pemasangan ke host lain lewat SSH
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
   | dropdown pertama kali digambar | ~1.600 |
   | pindah pilihan satu baris | ~430 |
   | render dengan isi identik | 4 |

   Garis bingkai tidak pernah berubah antar penekanan tombol, jadi ia hanya
   dikirim sekali untuk seluruh sesi meski dropdown digambar berkali-kali.

   Mode degradasi `UF_SIMPLE=1` mematikan warna dan sorotan, dan menyala
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

   Yang dirasakan pengguna lebih besar dari itu, karena `uf` adalah proses
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

Tiga satuan berbeda, jadi `uf widget` menerima `--cursor-unit rune\|byte\|utf16`.
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
| Interpreter selalu ditolak | `bash`, `sh`, `python`, `node`, `sudo`, `env`, `xargs` — argumennya adalah kode, bukan data |
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

```sh
UF_NO_GENERATORS=1               # matikan seluruhnya
UF_GENERATOR_ALLOW=tmux,kubectl  # izinkan biner tambahan
UF_GENERATOR_ALLOW_ROOT=1        # izinkan berjalan sebagai root
UF_GENERATOR_TIMEOUT=800ms       # ubah batas waktu
```

`uf complete --line "..."` menjalankan jalur yang sama persis dengan Tab, dan
mencetak alasan setiap penolakan ke stderr — jadi apa yang akan dijalankan uf
bisa diperiksa sebelum dipasang.

### Keluaran mentah

`postProcess` milik Fig berupa closure JavaScript dan tidak ikut ter-transpile,
sehingga keluaran perintah dinormalkan dengan aturan yang sedikit dan
eksplisit: tab atau dua spasi memisahkan nama dari keterangannya, penanda `* `
milik git dibuang, dan kandidat yang masih mengandung spasi dibuang seluruhnya
— teks yang tidak bisa disisipkan apa adanya menghasilkan perintah yang rusak,
bukan sekadar tampilan yang jelek.

### Cache

Keluaran generator disimpan di disk, bukan di memori: `uf` adalah proses baru
setiap kali Tab ditekan, jadi cache dalam memori tidak akan pernah terpakai
sekali pun. Kuncinya mencakup direktori kerja, karena `git branch` menjawab
berbeda di setiap repo.

## Windows

PowerShell 5.1 ke atas, dengan PSReadLine yang sudah menjadi bawaannya. Windows
Terminal mendukung VT penuh sejak 2019, jadi renderer yang sama langsung
berlaku; pada conhost lama `uf` menyalakan `ENABLE_VIRTUAL_TERMINAL_PROCESSING`
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
uf bootstrap deploy@web-01
```

Perintah itu mendeteksi platform host, mengalirkan binary dan spec lewat koneksi
SSH yang sama, lalu memeriksa hasilnya dengan benar-benar menjalankan berkas yang
dikirim. Menyalin bukan berarti bisa menjalankan: home yang dipasang `noexec`
baru ketahuan di langkah itu.

```
  host      : deploy@web-01 (linux/amd64)
  sumber    : uf_1.0.0_linux_amd64.tar.gz
  binary    : ~/.local/bin/uf
  spec      : ~/.local/share/uf/specs
  terkirim  : 6.6 MB
  terpasang : uf 1.0.0
```

Tata letaknya mengikuti XDG, sehingga penemuan spec berjalan tanpa variabel
lingkungan: `~/.local/bin/uf` mencari `../share/uf/specs` dan menemukannya.

### Yang TIDAK dilakukan perintah ini

**Bukan pembungkus `ssh`.** uf tidak pernah menyisip di antara kamu dan
koneksimu, tidak mengubah `~/.ssh/config`, dan tidak pernah berjalan otomatis
saat kamu menyambung ke suatu host. Perintah ini dijalankan sekali, dengan
sadar, lalu selesai.

Sebelum mengirim apa pun, rencananya ditampilkan dan persetujuan diminta.
Di luar terminal interaktif jawabannya selalu tidak — sebuah skrip tidak boleh
mendapat izin hanya karena tidak ada yang menjawab. Untuk pemakaian terskrip
ada dua jalan yang meninggalkan jejak: `--yes`, atau mendaftarkan host di
`~/.config/uf/hosts` — persetujuan yang bisa ditinjau dan disimpan di kendali
versi, bukan jawaban di layar yang menguap.

```
# ~/.config/uf/hosts
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
uf bootstrap --from dist web-01
```

Mesin lokal yang mengunduh, bukan host — jadi ini tetap bekerja untuk server
produksi yang tidak punya akses internet keluar.

Argumen setelah nama host diteruskan apa adanya ke `ssh`, sehingga `ProxyJump`,
bastion, dan opsi lain berlaku seperti biasa:

```sh
uf bootstrap web-01 -J bastion -i ~/.ssh/deploy
```

Perintah `ssh` sistem yang dipakai, bukan pustaka SSH — supaya seluruh isi
`~/.ssh/config` berlaku apa adanya, termasuk Match block dan kunci perangkat
keras. Koneksinya dibagi lewat `ControlMaster`: bootstrap memanggil host
beberapa kali, dan tanpa itu host ber-MFA akan meminta sentuhan kunci keamanan
berkali-kali untuk satu perintah.
