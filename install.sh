#!/bin/sh
# Pemasang anjuran untuk Linux dan macOS.
#
#   curl -fsSL https://raw.githubusercontent.com/ufhy/anjuran-cli/main/install.sh | sh
#
# Ditulis dalam sh POSIX, bukan bash: perintah di atas menjalankannya dengan
# /bin/sh, dan di Debian, Ubuntu, serta Alpine itu bukan bash. Skrip yang
# memakai bashism akan gagal justru di sistem yang paling banyak dipakai.
#
# TIDAK pernah memakai sudo. Semuanya dipasang di bawah rumah pengguna,
# mengikuti tata letak XDG, sehingga anjuran menemukan spec-nya tanpa
# variabel lingkungan: ia mencari ../share/anjuran/specs relatif terhadap
# binary-nya, dan bin/anjuran dengan share/anjuran/specs memenuhi itu persis.
#
# Lingkungan yang dibaca:
#   ANJURAN_VERSION     tag rilis tertentu, misalnya v0.1.0 (bawaan: terbaru)
#   ANJURAN_PRERELEASE  bila diisi, terima juga rilis beta
#   ANJURAN_INSTALL_DIR direktori dasar (bawaan: $HOME/.local)
#   ANJURAN_NO_SHELL    bila diisi, jangan sentuh berkas konfigurasi shell
#   ANJURAN_DRY_RUN     bila diisi, laporkan rencananya tanpa mengunduh apa pun
#   ANJURAN_BASE_URL    asal berkas rilis; untuk cermin, dan supaya skrip ini
#                       bisa diuji tanpa menerbitkan rilis sungguhan

set -eu

REPO="ufhy/anjuran-cli"
ASAL="${ANJURAN_BASE_URL:-https://github.com/$REPO/releases/download}"
BASE="${ANJURAN_INSTALL_DIR:-$HOME/.local}"

# Penanda yang sama dipakai `anjuran up`, supaya blok yang ditulis salah satu
# dikenali oleh yang lain dan tidak pernah ditumpuk dua kali.
PENANDA_AWAL="# >>> anjuran >>>"
PENANDA_AKHIR="# <<< anjuran <<<"

info() { printf '%s\n' "$*"; }
galat() { printf 'anjuran: %s\n' "$*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# Platform

platform() {
  os=$(uname -s)
  arch=$(uname -m)

  case $os in
    Linux)  os=linux ;;
    Darwin) os=darwin ;;
    *) galat "sistem $os belum didukung; untuk Windows pakai install.ps1" ;;
  esac

  # uname -m menyebut arsitektur yang sama dengan beberapa nama, dan nama
  # yang dipakai nama berkas rilis hanya satu.
  case $arch in
    x86_64|amd64)  arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) galat "arsitektur $arch belum didukung" ;;
  esac

  # Nilainya disetel sebagai variabel, bukan dicetak lalu dipecah kembali
  # oleh pemanggilnya: fungsi sh berjalan di shell yang sama, jadi mencetak
  # dua nilai hanya untuk memecahnya lagi menambah satu tempat kesalahan
  # tanpa menambah apa pun.
  ANJURAN_OS=$os
  ANJURAN_ARCH=$arch
}

# ---------------------------------------------------------------------------
# Pengunduh
#
# curl dan wget keduanya diterima: image container minimal sering punya
# hanya salah satunya, dan memaksakan satu berarti pemasangannya gagal di
# tempat yang justru paling sering dipakai untuk mencoba.

unduh() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1" -o "$2"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$2" "$1"
  else
    galat "butuh curl atau wget"
  fi
}

unduh_stdout() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "$1"
  else
    galat "butuh curl atau wget"
  fi
}

# tag_pertama mengambil tag_name pertama dari keluaran JSON.
#
# Diurai dengan sed, bukan jq: jq bukan bawaan di mana pun, dan menuntutnya
# berarti pemasangan gagal sebelum apa pun terunduh.
tag_pertama() {
  sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1
}

# versi_terbaru memilih rilis mana yang dipasang bila pengguna tidak menyebut.
#
# /releases/latest sengaja TIDAK memuat prarilis — itulah gunanya. Tetapi
# selama proyek ini belum punya rilis stabil, satu-satunya yang ada adalah
# beta, dan pemasang yang berkeras pada "latest" akan berkata tidak ada apa-apa
# padahal berkasnya ada. Jadi stabil dicoba lebih dulu, lalu jatuh ke rilis
# terbaru apa pun — dan pengguna diberi tahu bahwa yang dipasang sebuah beta,
# bukan dibiarkan mengiranya versi biasa.
versi_terbaru() {
  if [ -z "${ANJURAN_PRERELEASE:-}" ]; then
    stabil=$(unduh_stdout "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null | tag_pertama) || stabil=""
    if [ -n "$stabil" ]; then
      printf '%s\n' "$stabil"
      return 0
    fi
    info "  catatan  : belum ada rilis stabil; memakai prarilis terbaru" >&2
  fi
  unduh_stdout "https://api.github.com/repos/$REPO/releases?per_page=1" 2>/dev/null | tag_pertama
}

# ---------------------------------------------------------------------------
# Checksum
#
# Arsip diunduh lewat jaringan, dan yang dipasangnya adalah biner yang akan
# dijalankan setiap kali pengguna menekan Tab. Memeriksanya bukan kemewahan.
# Nama perintahnya berbeda antar sistem: sha256sum di Linux, shasum di macOS.

periksa_checksum() {
  arsip=$1
  daftar=$2
  nama=$(basename "$arsip")

  mau=$(sed -n "s/^\([0-9a-f]\{64\}\)[[:space:]]*\*\{0,1\}$nama$/\1/p" "$daftar" | head -1)
  if [ -z "$mau" ]; then
    galat "$nama tidak ada di checksums.txt"
  fi

  if command -v sha256sum >/dev/null 2>&1; then
    dapat=$(sha256sum "$arsip" | cut -d' ' -f1)
  elif command -v shasum >/dev/null 2>&1; then
    dapat=$(shasum -a 256 "$arsip" | cut -d' ' -f1)
  else
    info "  checksum : dilewati (tidak ada sha256sum maupun shasum)"
    return 0
  fi

  if [ "$dapat" != "$mau" ]; then
    galat "checksum tidak cocok untuk $nama
  mau   : $mau
  dapat : $dapat"
  fi
  info "  checksum : cocok"
}

# ---------------------------------------------------------------------------
# Konfigurasi shell

# berkas_shell memilih berkas konfigurasi berdasarkan shell LOGIN pengguna.
#
# Bukan dari $SHELL sesi ini: skrip ini dijalankan lewat pipe ke sh, jadi
# $SHELL di sini adalah sh — dan menulis ke berkas yang tidak pernah dibaca
# adalah cara paling halus untuk membuat pemasangan tampak berhasil padahal
# tidak ada yang berubah.
# shell_berjalan menebak shell yang BENAR-BENAR dipakai orangnya.
#
# Skrip ini dijalankan lewat pipe ke sh, jadi induk prosesnya adalah shell
# tempat perintah curl itu diketik — dan itu jawaban yang jauh lebih dekat
# dengan kenyataan daripada /etc/passwd. Di container passwd hampir tidak
# pernah disetel: root di sana tercatat memakai /bin/sh sementara orangnya
# duduk di dalam bash.
shell_berjalan() {
  induk=""
  # /proc dibaca lebih dulu, dan bukan sekadar sebagai jalan pintas: `ps` milik
  # BusyBox tidak mengenal opsi -p sama sekali, sehingga di Alpine — tempat
  # kekeliruan ini paling sering terjadi — jalur ps tidak pernah menjawab.
  if [ -r "/proc/$PPID/comm" ]; then
    induk=$(cat "/proc/$PPID/comm" 2>/dev/null)
  fi
  if [ -z "$induk" ]; then
    induk=$(ps -o comm= -p "$PPID" 2>/dev/null) || induk=""
  fi
  induk=${induk##*/}
  # Shell login muncul dengan awalan tanda hubung, misalnya "-bash".
  printf '%s\n' "${induk#-}"
}

# shell_login membaca shell yang tercatat di passwd.
shell_login() {
  masuk=""
  if command -v getent >/dev/null 2>&1; then
    masuk=$(getent passwd "$(id -un)" 2>/dev/null | cut -d: -f7)
  fi
  if [ -z "$masuk" ]; then
    masuk=$(id -P "$(id -un)" 2>/dev/null | cut -d: -f10) || masuk=""
  fi
  [ -n "$masuk" ] || masuk=${SHELL:-}
  basename "${masuk:-sh}"
}

# berkas_shell memilih berkas konfigurasi mana yang ditulis.
#
# Yang sedang dijalankan didahulukan atas yang tercatat di passwd, karena di
# situlah orangnya benar-benar mengetik. Keduanya diperiksa, dan yang pertama
# DIDUKUNG yang dipakai: shell yang sedang berjalan bisa saja sh — misalnya
# saat skrip ini dipanggil dari skrip lain — dan di situ passwd lebih tahu.
berkas_shell() {
  for kandidat in "$(shell_berjalan)" "$(shell_login)"; do
    case $kandidat in
      zsh)  RC_BERKAS="$HOME/.zshrc";                   RC_JENIS=zsh;  return 0 ;;
      bash) RC_BERKAS="$HOME/.bashrc";                  RC_JENIS=bash; return 0 ;;
      fish) RC_BERKAS="$HOME/.config/fish/config.fish"; RC_JENIS=fish; return 0 ;;
    esac
  done
  RC_BERKAS="$HOME/.profile"
  RC_JENIS=polos
}

pasang_shell() {
  rc=$1
  jenis=$2
  bindir="\$HOME/${BASE#"$HOME"/}/bin"

  if [ -f "$rc" ] && grep -qF "$PENANDA_AWAL" "$rc" 2>/dev/null; then
    info "  shell    : $rc (sudah ada)"
    return 0
  fi

  mkdir -p "$(dirname "$rc")"
  # shellcheck disable=SC2016
  # Kutip tunggalnya disengaja. Yang ditulis ke berkas rc harus berupa $HOME
  # dan $PATH LITERAL, supaya barisnya tetap benar kalau rumah pengguna
  # pindah — bukan nilainya pada saat pemasangan.
  {
    printf '\n%s\n' "$PENANDA_AWAL"
    case $jenis in
      fish)
        printf 'set -gx PATH %s $PATH\n' "$bindir"
        printf 'anjuran init fish | source\n'
        ;;
      zsh|bash)
        printf 'export PATH="%s:$PATH"\n' "$bindir"
        printf 'eval "$(anjuran init %s)"\n' "$jenis"
        ;;
      *)
        # sh, ash, dash, ksh: tidak ada integrasi untuk dipasang di sana.
        # PATH tetap disetel, karena tanpa itu binary yang barusan dipasang
        # tidak bisa dipanggil sama sekali.
        printf 'export PATH="%s:$PATH"\n' "$bindir"
        ;;
    esac
    printf '%s\n' "$PENANDA_AKHIR"
  } >> "$rc"

  if [ "$jenis" = polos ]; then
    info "  shell    : $rc (PATH saja)"
  else
    info "  shell    : $rc (ditambahkan)"
  fi
}

# peringatan_tanpa_integrasi menjelaskan keadaan yang paling mudah disalahpahami.
#
# "PATH saja" terbaca seperti keberhasilan, padahal fitur utamanya justru tidak
# terpasang: binary-nya ada, tetapi menekan spasi tidak akan memunculkan apa
# pun. Yang dicetak di sini perintah yang bisa langsung ditempel, bukan
# keterangan bahwa sesuatu tidak didukung.
peringatan_tanpa_integrasi() {
  bindir="\$HOME/${BASE#"$HOME"/}/bin"
  info ""
  info "Shell-mu tidak dikenali, jadi HANYA PATH yang disetel — kotak"
  info "completion belum akan muncul. Kalau kamu memakai bash atau zsh,"
  info "tambahkan satu baris ini sendiri:"
  info ""
  info "  echo 'export PATH=\"$bindir:\$PATH\"' >> ~/.bashrc"
  info "  echo 'eval \"\$(anjuran init bash)\"' >> ~/.bashrc"
  info ""
  info "Ganti bash dengan zsh dan ~/.bashrc dengan ~/.zshrc bila perlu."
}

# ---------------------------------------------------------------------------

main() {
  platform
  os=$ANJURAN_OS
  arch=$ANJURAN_ARCH

  versi=${ANJURAN_VERSION:-}
  if [ -z "$versi" ]; then
    versi=$(versi_terbaru) || true
    [ -n "$versi" ] || galat "tidak bisa membaca rilis terbaru dari GitHub; sebutkan ANJURAN_VERSION"
  fi

  # Nama berkas rilis memakai versi TANPA awalan v, sedangkan tag memakainya.
  polos_versi=${versi#v}
  nama="anjuran_${polos_versi}_${os}_${arch}.tar.gz"
  url="$ASAL/$versi/$nama"

  info "Memasang anjuran:"
  info ""
  info "  versi    : $versi"
  info "  platform : $os/$arch"
  info "  dari     : $url"
  info "  ke       : $BASE/bin/anjuran"
  info ""

  if [ -n "${ANJURAN_DRY_RUN:-}" ]; then
    info "Mode dry-run; tidak ada yang diunduh."
    return 0
  fi

  tmp=$(mktemp -d 2>/dev/null || mktemp -d -t anjuran)
  # Dibersihkan apa pun yang terjadi: arsip 7 MB yang tertinggal di /tmp
  # setiap kali pemasangan gagal adalah kerusakan yang tidak diminta siapa pun.
  trap 'rm -rf "$tmp"' EXIT INT TERM

  unduh "$url" "$tmp/$nama" ||
    galat "gagal mengunduh $url
  Periksa apakah rilis $versi memang punya berkas untuk $os/$arch."

  if unduh "$ASAL/$versi/checksums.txt" "$tmp/checksums.txt" 2>/dev/null; then
    periksa_checksum "$tmp/$nama" "$tmp/checksums.txt"
  else
    info "  checksum : dilewati (checksums.txt tidak ada di rilis ini)"
  fi

  tar xzf "$tmp/$nama" -C "$tmp" || galat "arsipnya tidak bisa dibongkar"
  [ -f "$tmp/anjuran" ] || galat "arsipnya tidak memuat binary anjuran"

  mkdir -p "$BASE/bin" "$BASE/share/anjuran"
  install -m 0755 "$tmp/anjuran" "$BASE/bin/anjuran" 2>/dev/null || {
    cp "$tmp/anjuran" "$BASE/bin/anjuran"
    chmod 0755 "$BASE/bin/anjuran"
  }

  # Spec lama dibuang lebih dulu agar berkas yang sudah tidak ada di rilis
  # baru tidak tertinggal dan tetap ditawarkan.
  for d in specs extra; do
    if [ -d "$tmp/$d" ]; then
      rm -rf "$BASE/share/anjuran/$d"
      cp -R "$tmp/$d" "$BASE/share/anjuran/$d"
    fi
  done

  # macOS mengarantina berkas yang diunduh; tanpa dilepas, Gatekeeper
  # menolak menjalankannya dan gejalanya bukan pesan izin melainkan
  # "killed".
  if [ "$os" = darwin ] && command -v xattr >/dev/null 2>&1; then
    xattr -dr com.apple.quarantine "$BASE/bin/anjuran" 2>/dev/null || true
  fi

  terpasang=$("$BASE/bin/anjuran" version 2>/dev/null || echo "")
  [ -n "$terpasang" ] || galat "binary tersalin tetapi tidak bisa dijalankan; biasanya karena rumah dipasang noexec"
  info "  terpasang: $terpasang"

  jumlah=$(find "$BASE/share/anjuran/specs" -name '*.json.gz' 2>/dev/null | wc -l | tr -d ' ')
  info "  spec     : $jumlah"

  if [ -z "${ANJURAN_NO_SHELL:-}" ]; then
    berkas_shell
    pasang_shell "$RC_BERKAS" "$RC_JENIS"
  fi

  info ""
  if [ "${RC_JENIS:-}" = polos ]; then
    peringatan_tanpa_integrasi
  else
    info "Buka sesi shell baru, lalu tekan spasi sesudah sebuah perintah."
    # Sesi yang SEDANG berjalan tidak membaca ulang berkas rc-nya sendiri.
    # Tanpa disebut, perintah berikutnya yang orang ketik adalah `anjuran`,
    # dan jawabannya "command not found" — tepat sesudah pemasangan yang
    # dilaporkan berhasil.
    info "Untuk sesi ini: . $RC_BERKAS"
  fi
}

main "$@"
