# Integrasi anjuran untuk bash.
#
# Pasang dengan menambahkan satu baris ini ke ~/.bashrc:
#
#     eval "$(anjuran init bash)"
#
# Membutuhkan bash 4.0 atau lebih baru, karena READLINE_LINE dan READLINE_POINT
# baru ada sejak versi itu. Bash 3.2 bawaan macOS tidak didukung.
#
# Tombol pemicu bisa diganti lewat ANJURAN_KEY sebelum eval, misalnya:
#
#     ANJURAN_KEY='\C-@' eval "$(anjuran init bash)"    # Ctrl-Spasi, Tab tetap milik bash

# Shell non-interaktif tidak punya readline; jangan pasang apa pun.
case $- in *i*) ;; *) return 0 ;; esac

if [ -z "${BASH_VERSINFO:-}" ] || [ "${BASH_VERSINFO[0]}" -lt 4 ]; then
  printf 'anjuran: butuh bash 4.0+, terpasang %s\n' "${BASH_VERSION:-tidak diketahui}" >&2
  return 0
fi

# _anjuran_alias mencari arti alias untuk kata pertama segmen TERAKHIR.
#
# Alias hanya berlaku di posisi perintah, jadi "docker ps | gst" memakai gst,
# bukan docker. Tanpa ini sebuah alias sama sekali tidak dikenali dan
# completion untuk perintah di baliknya tidak pernah muncul.
_anjuran_alias() {
  _anjuran_alias_exp=""

  local -a kata
  read -r -a kata <<< "$READLINE_LINE"
  [ ${#kata[@]} -eq 0 ] && return

  local w pertama=""
  for w in "${kata[@]}"; do
    case $w in
      '|'|'||'|'&&'|';'|'&') pertama=""; continue ;;
    esac
    [ -z "$pertama" ] && pertama=$w
  done
  [ -n "$pertama" ] || return

  # `alias -p` mencetak "alias nama='isi'"; yang dibutuhkan hanya isinya.
  local baris
  baris=$(alias -p 2>/dev/null | command grep -m1 "^alias $pertama=") || return
  baris=${baris#alias $pertama=}
  # Lepas kutip pembungkusnya.
  case $baris in
    \'*\') baris=${baris#\'}; baris=${baris%\'} ;;
    '"'*'"') baris=${baris#\"}; baris=${baris%\"} ;;
  esac
  _anjuran_alias_exp=$baris
}

# _anjuran_gambar_baris menggambar ulang prompt dan baris perintah.
#
# bash MENGHAPUS baris yang terlihat sebelum menjalankan perintah `bind -x`,
# lalu menggambarnya ulang sesudah perintah itu selesai. Untuk perintah biasa
# itu benar. Untuk anjuran tidak: kotaknya digambar SELAMA fungsi berjalan,
# jadi selama kotak terbuka prompt dan baris perintahnya memang kosong di
# layar — mengetik "cd /" menampilkan daftar direktori tanpa satu pun jejak
# perintah yang sedang ditulis.
#
# Buffer-nya sendiri selalu benar, dan itu yang membuat cacat seperti ini
# bertahan lama: perintahnya tetap berjalan sebagaimana mestinya. zsh
# mengatasi hal yang sama dengan `zle -R`; readline tidak punya padanan yang
# bisa dipanggil dari dalam `bind -x`.
_anjuran_gambar_baris() {
  local prompt=""
  # ${PS1@P} memekarkan escape prompt seperti \u dan \w; baru ada di bash 4.4.
  # Di bawah itu barisnya tetap digambar, hanya tanpa prompt — masih jauh
  # lebih baik daripada layar yang kosong.
  if [ "${BASH_VERSINFO[0]}" -gt 4 ] ||
     { [ "${BASH_VERSINFO[0]}" -eq 4 ] && [ "${BASH_VERSINFO[1]}" -ge 4 ]; }; then
    prompt=${PS1@P}
    # \[ dan \] menjadi byte 0x01 dan 0x02 setelah pemekaran. Keduanya
    # penanda "lebar nol" untuk readline, bukan sesuatu yang boleh sampai ke
    # terminal.
    prompt=${prompt//$'\001'/}
    prompt=${prompt//$'\002'/}
  fi
  # Baris DIHAPUS dulu, bukan sekadar ditimpa.
  #
  # Widget ini bisa berjalan dua kali untuk satu penekanan tombol: memilih
  # sebuah folder menyisipkan namanya, lalu membuka isinya dengan memanggil
  # widget sekali lagi. Tanpa penghapusan, gambar kedua mendarat di sebelah
  # gambar pertama dan barisnya terbaca ganda:
  #
  #   bash-5.3# cd home/bash-5.3# cd home/
  printf '\r\033[K%s%s' "$prompt" "$READLINE_LINE" > /dev/tty 2>/dev/null
}

_anjuran_widget() {
  local trigger=${1:-manual}
  local select_from=${2:-first}
  local out head body anjuran_status new_cursor sisa
  # Baris SEBELUM sesi dijalankan, untuk tahu apakah ada yang tersisip.
  local sebelum=$READLINE_LINE
  local _anjuran_alias_exp
  _anjuran_alias
  _anjuran_gambar_baris

  # READLINE_POINT dihitung dalam BYTE, berbeda dari zsh dan fish yang
  # menghitung karakter. Salah satuan akan menyisipkan di tempat yang salah
  # begitu baris memuat huruf non-ASCII.
  out="$(command anjuran widget --line "$READLINE_LINE" --cursor "$READLINE_POINT" \
    --cursor-unit byte --trigger "$trigger" --select "$select_from" \
    --alias "$_anjuran_alias_exp" 2>/dev/null)"

  # Keluaran kosong berarti anjuran mati di tengah jalan. Menyisipkan sesuatu
  # sebagai gantinya hanya benar bila pengguna memang MEMINTA penyisipan, yaitu
  # saat ia menekan Tab. Pada pemicu otomatis, tidak ada yang boleh muncul.
  if [ -z "$out" ]; then
    [ "$trigger" = manual ] && _anjuran_fallback
    return
  fi

  head=${out%%$'\n'*}
  if [ "$out" != "${out%$'\n'*}" ]; then
    body=${out#*$'\n'}
  else
    body=''
  fi

  anjuran_status=${head%% *}
  # head berbentuk "<status> <kursor> <sisa-heksadesimal>".
  local _ekor=${head#* }
  new_cursor=${_ekor%% *}
  sisa=${_ekor##* }

  case $anjuran_status in
    ok)
      READLINE_LINE=$body
      READLINE_POINT=$new_cursor

      # Memilih sebuah direktori membuka isinya.
      #
      # Garis miring adalah karakter pemicu, sama seperti spasi — dan yang
      # baru saja disisipkan memang garis miring. Bahwa ia datang dari pilihan
      # pengguna alih-alih diketik tidak mengubah apa pun.
      #
      # Dibuka TANPA ada yang tersorot, supaya Enter berarti "cukup, pakai
      # path ini" dan penelusuran punya cara berhenti. Kutip penutup dilepas
      # dulu, karena nama berspasi disisipkan terkutip.
      local _ekor_baris=${READLINE_LINE%[\'\"]}
      if [ "$READLINE_LINE" != "$sebelum" ] && [ "${_ekor_baris: -1}" = "/" ] && [ -z "$sisa" ]; then
        _anjuran_widget "$trigger" none
      fi
      ;;
    none)
      [ "$trigger" = manual ] && _anjuran_fallback
      ;;
    *)
      # Dibatalkan: biarkan baris apa adanya.
      ;;
  esac
}

# _anjuran_fallback melengkapi nama berkas saat anjuran tidak punya jawaban.
#
# Ini memang tidak sekuat completion bawaan bash: `bind -x` mengambil alih
# tombolnya sepenuhnya, dan tidak ada cara memanggil kembali fungsi yang
# didaftarkan `complete` dari dalamnya. Karena itu yang ditiru hanya perilaku
# bawaan readline, yaitu melengkapi path — kasus yang paling sering muncul
# untuk perintah tanpa spec.
#
# Siapa pun yang lebih membutuhkan bash-completion daripada anjuran sebaiknya
# memindahkan pemicunya: ANJURAN_KEY='\C-@' eval "$(anjuran init bash)"
_anjuran_fallback() {
  local word prefix matches
  word=${READLINE_LINE:0:$READLINE_POINT}
  word=${word##* }
  prefix=${READLINE_LINE:0:$((READLINE_POINT - ${#word}))}

  mapfile -t matches < <(compgen -o default -- "$word" 2>/dev/null)
  [ ${#matches[@]} -eq 0 ] && return

  local insert=${matches[0]}
  if [ ${#matches[@]} -gt 1 ]; then
    # Banyak kandidat: sisipkan awalan terpanjang yang sama, seperti readline.
    local i common=${matches[0]}
    for i in "${matches[@]}"; do
      while [ "${i#"$common"}" = "$i" ] && [ -n "$common" ]; do
        common=${common%?}
      done
    done
    insert=$common
    [ -z "$insert" ] && return
  fi

  READLINE_LINE="$prefix$insert${READLINE_LINE:$READLINE_POINT}"
  READLINE_POINT=$((${#prefix} + ${#insert}))
}

bind -x "\"${ANJURAN_KEY:-\C-i}\": _anjuran_widget"

# ---------------------------------------------------------------------------
# Pemicu otomatis
#
# Kotak muncul begitu sebuah kata selesai ditulis, bukan sambil mengetik:
# spasi menutup sebuah kata, / menutup sebuah komponen path, = menutup nama
# sebuah opsi. Ketiganya sama persis dengan yang dipasang integrasi zsh —
# pemicu yang berbeda antar shell membuat alat yang sama terasa seperti dua
# alat berbeda begitu seseorang berpindah mesin.
#
# NYALA secara bawaan. Matikan dengan ANJURAN_AUTO=0.

_anjuran_sisip() {
  READLINE_LINE="${READLINE_LINE:0:$READLINE_POINT}$1${READLINE_LINE:$READLINE_POINT}"
  READLINE_POINT=$((READLINE_POINT + ${#1}))
}

# _anjuran_pemicu menyisipkan karakternya sendiri, lalu membuka sesi.
#
# Menyisipkan sendiri adalah keharusan, bukan pilihan: `bind -x` mengambil alih
# tombolnya sepenuhnya, jadi tanpa ini karakter yang diketik pengguna hilang.
_anjuran_pemicu() {
  _anjuran_sisip "$1"

  # Jangan membuka kotak selagi masih ada ketikan yang menunggu dibaca.
  #
  # Menempel satu baris panjang mengirim seluruhnya sekaligus. Tanpa penjagaan
  # ini setiap spasi di dalam tempelan membuka sesi baru yang menunggu tombol
  # yang tidak akan pernah datang, dan shell terkunci. `read -t 0` menjawab
  # "apakah ada masukan yang siap" tanpa mengambilnya.
  if read -t 0 2>/dev/null; then
    return 0
  fi

  _anjuran_widget auto
}

_anjuran_spasi()       { _anjuran_pemicu ' '; }
_anjuran_garismiring() { _anjuran_pemicu '/'; }
_anjuran_samadengan()  { _anjuran_pemicu '='; }

case ${ANJURAN_AUTO:-1} in
  0|no|off|false) ;;
  *)
    bind -x '" ": _anjuran_spasi'
    bind -x '"/": _anjuran_garismiring'
    bind -x '"=": _anjuran_samadengan'
    ;;
esac

# ---------------------------------------------------------------------------
# Saran dari riwayat (ghost text): TIDAK ADA di bash, dan tidak bisa ada.
#
# Teks abu-abu yang melanjutkan ketikan membutuhkan dua hal yang readline
# tidak punya. Pertama, kait pada setiap ketikan: zsh membungkus `self-insert`,
# sedangkan di readline satu-satunya cara adalah mengikat kesembilan puluh
# lima karakter cetak satu per satu — mahal, dan merusak binding orang lain.
# Kedua, tempat menggambar teks yang BUKAN bagian dari buffer: zsh punya
# POSTDISPLAY, readline tidak. Menaruh sarannya di READLINE_LINE berarti
# menulis teks yang tidak diminta ke dalam perintah yang akan dijalankan.
#
# Jadi ANJURAN_GHOST tidak berpengaruh di bash. Disebutkan di sini supaya
# ketiadaannya menjadi keterangan, bukan kejutan. fish dan PowerShell memakai
# fitur bawaan shell masing-masing; zsh memakai implementasi sendiri.
