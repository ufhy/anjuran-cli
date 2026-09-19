# Integrasi anjuran untuk zsh.
#
# Pasang dengan menambahkan satu baris ini ke ~/.zshrc:
#
#     eval "$(anjuran init zsh)"
#
# Tombol pemicu bisa diganti lewat ANJURAN_KEY sebelum eval, misalnya:
#
#     ANJURAN_KEY='^ ' eval "$(anjuran init zsh)"     # Ctrl-Spasi, Tab tetap bawaan zsh

# Jangan pasang apa pun bila binary-nya tidak ada, agar .zshrc tetap aman
# disalin ke mesin yang belum terpasang anjuran.
(( $+commands[anjuran] )) || return 0

# _anjuran_alias menaruh pemekaran alias kata pertama ke dalam _anjuran_alias_exp.
#
# Tanpa ini alias sama sekali tidak dikenali: anjuran mencari spec bernama "gco" dan
# tidak menemukannya. Padahal alias justru cara sehari-hari orang memakai
# perintah panjang, dan oh-my-zsh memasang ratusan di antaranya.
#
# Nilainya diletakkan di variabel, bukan dicetak, supaya tidak menumbuhkan
# subshell pada jalur yang dijalankan setiap kali spasi ditekan.
typeset -g _anjuran_alias_exp

_anjuran_alias() {
  emulate -L zsh
  setopt local_options no_ksh_arrays

  _anjuran_alias_exp=""

  local -a words
  words=(${(z)BUFFER})
  (( $#words )) || return

  # Alias hanya berlaku di posisi perintah, jadi yang dicari adalah kata
  # pertama dari segmen TERAKHIR: "docker ps | gst" memakai gst, bukan docker.
  local w first=""
  for w in $words; do
    case $w in
      '|'|'||'|'&&'|';'|'&') first=""; continue ;;
    esac
    [[ -z $first ]] && first=$w
  done

  [[ -n $first ]] || return
  _anjuran_alias_exp=${aliases[$first]-}
}

# _anjuran_widget menjalankan SATU sesi yang memegang seluruh interaksi.
#
# Model ini mengikuti cara IDE bekerja: daftar kandidat diambil sekali, lalu
# disaring di tempat sambil pengguna mengetik — bukan dihitung ulang dari nol
# pada setiap ketikan. Satu proses per interaksi, bukan satu proses per huruf.
#
# Konsekuensinya sesi memegang masukan selama dropdown terbuka. Agar tidak ada
# yang dirampas dari zsh, tombol yang bukan urusan dropdown dikembalikan lewat
# zle -U dan diproses zsh seperti biasa.
_anjuran_widget() {
  emulate -L zsh
  setopt local_options no_ksh_arrays

  local out head body status_word new_cursor sisa
  # Baris SEBELUM sesi dijalankan, untuk tahu apakah ada yang tersisip.
  local sebelum=$BUFFER
  local select_from=${1:-first}
  # Pemicu otomatis tidak pernah berarti "sisipkan yang ini"; hanya Tab yang
  # berarti begitu.
  local trigger=${2:-manual}

  # Bayangan disembunyikan selama dropdown terbuka: dua saran sekaligus hanya
  # menambah kebisingan, dan teks setelah kursor mengganggu gambar kotaknya.
  if (( $+functions[_anjuran_ghost_hapus] )); then
    _anjuran_ghost_hapus
  fi

  # Baris digambar ulang LEBIH DULU, dan harus dengan `zle -R`.
  #
  # Karakter pemicu baru saja disisipkan ke buffer, tetapi zsh belum
  # menampilkannya — ia menggambar setelah widget selesai. Tanpa ini anjuran
  # mulai menggambar dari kolom yang salah, dan karakter pemicunya tidak pernah
  # terlihat.
  #
  # `zle redisplay` TIDAK cukup, walaupun namanya menjanjikan begitu: ia
  # menandai baris perlu digambar ulang, dan penggambarannya tetap menunggu
  # widget selesai — padahal saat itu anjuran sudah terlanjur menggambar
  # kotaknya. Akibatnya satu huruf hilang dari layar setiap kali kotak dibuka:
  # mengetik "git" menampilkan "gt", dan "cd" menampilkan "c". Buffer-nya
  # sendiri selalu benar, sehingga perintahnya tetap berjalan sebagaimana
  # mestinya — itu pula yang membuat cacat ini bertahan lama. `zle -R`
  # menggambar saat itu juga.
  zle -R

  _anjuran_alias
  # Dropdown digambar anjuran langsung ke /dev/tty; stdout hanya membawa hasil.
  out="$(command anjuran widget --line "$BUFFER" --cursor "$CURSOR" \
    --select "$select_from" --trigger "$trigger" --sisa-balik \
    --alias "$_anjuran_alias_exp" 2>/dev/null)"

  if [[ -z $out ]]; then
    # anjuran tidak bisa menjalankan sesi, misalnya karena bukan terminal
    # interaktif — atau karena ia jatuh. Untuk Tab, completion bawaan zsh jauh
    # lebih baik daripada tidak ada apa-apa.
    #
    # Untuk pemicu otomatis TIDAK: menekan spasi lalu tiba-tiba mendapat
    # completion zsh mengubah baris perintah tanpa diminta. Bila anjuran jatuh,
    # ia menyisipkan sesuatu yang tidak pernah dipilih siapa pun — dan
    # kegagalan yang seharusnya tidak terlihat justru menjadi perubahan baris.
    if [[ $trigger == manual ]]; then
      zle expand-or-complete
    fi
    return
  fi

  head=${out%%$'\n'*}
  if [[ $out == *$'\n'* ]]; then
    body=${out#*$'\n'}
  else
    body=''
  fi

  status_word=${${(z)head}[1]}
  new_cursor=${${(z)head}[2]}
  sisa=${${(z)head}[3]}

  case $status_word in
    ok)
      BUFFER=$body
      CURSOR=$new_cursor
      # Memilih sebuah direktori membuka isinya.
      #
      # Garis miring adalah karakter pemicu, sama seperti spasi — dan yang
      # baru saja disisipkan memang garis miring. Bahwa ia datang dari pilihan
      # pengguna alih-alih diketik tidak mengubah apa pun: memilih folder
      # jelas berarti "lanjutkan ke dalamnya". Sempat dibatasi hanya untuk
      # Tab, dan akibatnya menelusuri path terasa canggung — sesudah memilih
      # "Documents/" pengguna harus menghapus garis miringnya lalu
      # mengetiknya lagi hanya untuk memunculkan daftar berikutnya.
      #
      # Dibuka TANPA ada yang tersorot. Sebelumnya isinya dibuka dengan anak
      # pertama terpilih, sehingga menelusuri satu direktori langsung menyeret
      # ke dalam anaknya — dan bila anaknya tunggal, disisipkan lalu ditelusuri
      # lagi, turun terus sampai dasar tanpa pernah menawarkan cara berhenti.
      # Dengan "none", Enter berarti "cukup, pakai path ini".
      #
      # Kutip penutup dilepas dulu: nama berspasi disisipkan terkutip, sehingga
      # "cd 'folder dengan spasi/'" berakhir dengan kutip, bukan garis miring.
      # Tanpa ini folder berspasi tidak pernah bisa ditelusuri.
      # Hanya bila ada yang benar-benar TERSISIP.
      #
      # Enter di dalam kotak yang tidak menyorot apa pun berarti "cukup, pakai
      # path ini" dan mengembalikan baris yang sama persis. Tanpa syarat ini,
      # baris yang sudah berakhir garis miring akan membuka kotaknya lagi pada
      # setiap Enter — dan tidak ada cara berhenti sama sekali.
      local _anjuran_ekor=${BUFFER%[\'\"]}
      if [[ $BUFFER != $sebelum && $_anjuran_ekor == */ && -z $sisa ]]; then
        zle -R
        _anjuran_widget none "$trigger"
        return
      fi
      ;;
    none)
      # Tidak ada yang bisa ditawarkan.
      #
      # Untuk Tab, completion bawaan zsh masih jauh lebih baik daripada tidak
      # ada apa-apa. Untuk karakter pemicu TIDAK: mengetik spasi lalu tiba-tiba
      # mendapat completion zsh mengubah baris perintah tanpa diminta.
      if [[ $trigger == manual ]]; then
        zle expand-or-complete
      fi
      return
      ;;
    cancel|*)
      # Biarkan buffer apa adanya.
      ;;
  esac

  zle -R
  _anjuran_kembalikan "$sisa"

  # Bayangan dihitung ulang untuk baris yang baru.
  if (( $+functions[_anjuran_ghost_perbarui] )); then
    _anjuran_ghost_perbarui
  fi
}

# _anjuran_kembalikan mengembalikan tombol yang belum ditangani ke antrean masukan
# zsh, sehingga diproses seperti tidak pernah lewat anjuran.
#
# Dikirim sebagai heksadesimal karena isinya byte kendali yang tidak aman
# dilewatkan apa adanya di dalam satu baris teks.
_anjuran_kembalikan() {
  local hex=$1
  [[ -n $hex && $hex != 0 ]] || return 0

  local esc="" teks="" i
  for (( i = 1; i <= ${#hex}; i += 2 )); do
    esc+="\\x${hex[i,i+1]}"
  done

  # print tanpa -r menafsirkan \xHH, dan -v menaruh hasilnya ke variabel tanpa
  # subshell. Substitusi perintah tidak dipakai: ia membuang newline di ujung,
  # dan byte yang dikembalikan bisa berisi apa saja.
  print -v teks -n -- "$esc"
  [[ -n $teks ]] && zle -U -- "$teks"
}

zle -N _anjuran_widget

# ---------------------------------------------------------------------------
# Pemicu
#
# Tab selalu membuka sesi. Selain itu, kotak muncul sendiri pada karakter yang
# MENGAKHIRI sebuah kata — spasi, "/", dan "=" — karena di situlah kata itu
# selesai dan ada sesuatu yang bisa ditawarkan untuk berikutnya.
#
# Mengetik huruf TIDAK membuka kotak. Sempat dibuat begitu, meniru editor yang
# memunculkan daftar sambil nama diketik, dan di shell itu terasa mengganggu:
# kotak berkedip pada hampir setiap kata, termasuk kata yang jelas bukan
# perintah. Baris perintah bukan berkas kode — ia pendek, ditulis sekali, dan
# lebih sering diketik habis daripada dijelajahi. Menunggu spasi membuat
# saran datang tepat saat pengguna berhenti, bukan sambil ia mengetik.
#
# Nama perintah tetap bisa dilengkapi — dengan Tab, yang memang berarti
# "tolong lengkapi".
#
# NYALA secara bawaan. Matikan dengan ANJURAN_AUTO=0.
# ---------------------------------------------------------------------------

bindkey "${ANJURAN_KEY:-^I}" _anjuran_widget

if [[ ${ANJURAN_AUTO:-1} != (0|no|off|false) ]]; then
  # Widget asli yang terpasang pada sebuah tombol, supaya perilakunya tetap
  # utuh sebelum sesi dibuka. oh-my-zsh memetakan spasi ke magic-space, yang
  # memekarkan rujukan riwayat lebih dulu.
  typeset -gA _anjuran_asli

  _anjuran_simpan_asli() {
    local key=$1 nama=$2 keluaran orig
    keluaran="$(bindkey "$key")"
    orig=${keluaran##* }
    if [[ -n $orig && $orig != undefined-key ]]; then
      _anjuran_asli[$nama]=$orig
    fi
  }

  # _anjuran_simpan_asli_widget menyimpan widget yang SUDAH terpasang di sebuah
  # nama, supaya pembungkus milik plugin lain tetap dipanggil.
  #
  # zsh-autosuggestions dan zsh-syntax-highlighting juga membungkus self-insert.
  # Memanggil `zle .self-insert` begitu saja akan melompati keduanya, dan
  # keduanya berhenti bekerja tanpa pesan apa pun.
  _anjuran_simpan_asli_widget() {
    local nama=$1 w=${widgets[$1]}
    case $w in
      user:*)      _anjuran_asli[$nama]=${w#user:} ;;
      builtin|'')  ;;
    esac
  }

  # _anjuran_asal menjalankan widget yang sudah terpasang, atau builtin-nya.
  #
  # Keputusannya berdasarkan ADA atau tidaknya widget itu, bukan berdasarkan
  # status kembaliannya. Widget bisa mengembalikan status bukan-nol untuk
  # keadaan yang wajar — backward-delete-char melakukannya saat kursor sudah di
  # awal baris — dan memakai status sebagai syarat akan membuat builtin-nya
  # ikut dijalankan sesudahnya: satu penekanan tombol, dua tindakan.
  _anjuran_asal() {
    local kunci=$1 diri=$2 bawaan=$3
    local orig=${_anjuran_asli[$kunci]}
    if [[ -n $orig && $orig != $diri && -n ${widgets[$orig]} ]]; then
      zle "$orig"
      return
    fi
    zle "$bawaan"
  }

  # _anjuran_pemicu menjalankan widget asli tombolnya, lalu membuka sesi.
  _anjuran_pemicu() {
    local nama=$1
    _anjuran_asal $nama $nama .self-insert
    # Jangan membuka kotak selagi masih ada ketikan yang menunggu dibaca.
    #
    # Menempel satu baris panjang mengirim seluruhnya sekaligus; zsh membacanya
    # ke penyangganya sendiri lalu memprosesnya satu per satu. Tanpa penjagaan
    # ini setiap spasi di dalam tempelan membuka sesi baru yang menunggu tombol
    # yang tidak akan pernah datang — tombolnya sudah ada di penyangga zsh,
    # bukan di terminal. Hasilnya shell terkunci. Sekaligus benar untuk
    # mengetik cepat: kotak yang digambar dari baris yang sudah basi hanya
    # mengganggu.
    (( PENDING + KEYS_QUEUED_COUNT == 0 )) || return 0
    _anjuran_widget first auto
    return 0
  }

  _anjuran_spasi()      { _anjuran_pemicu _anjuran_spasi }
  _anjuran_garismiring(){ _anjuran_pemicu _anjuran_garismiring }
  _anjuran_samadengan() { _anjuran_pemicu _anjuran_samadengan }
  zle -N _anjuran_spasi
  zle -N _anjuran_garismiring
  zle -N _anjuran_samadengan

  _anjuran_simpan_asli " " _anjuran_spasi
  _anjuran_simpan_asli "/" _anjuran_garismiring
  _anjuran_simpan_asli "=" _anjuran_samadengan

  bindkey " " _anjuran_spasi
  bindkey "/" _anjuran_garismiring
  bindkey "=" _anjuran_samadengan
  if [[ -n ${keymaps[(r)viins]} ]]; then
    bindkey -M viins " " _anjuran_spasi
    bindkey -M viins "/" _anjuran_garismiring
    bindkey -M viins "=" _anjuran_samadengan
  fi

fi

# ---------------------------------------------------------------------------
# Saran dari riwayat (ghost text)
#
# Teks abu-abu yang melanjutkan ketikan berdasarkan perintah yang pernah
# dijalankan. Untuk perintah panjang yang diulang setiap hari — kubectl logs
# dengan namespace dan selector — ini lebih sering menolong daripada dropdown.
#
# Seluruhnya dikerjakan di dalam zsh, tanpa memanggil anjuran sama sekali: ia harus
# diperbarui pada SETIAP ketikan, dan menumbuhkan proses di sana akan terasa
# berat. anjuran hanya menggambar dropdown; riwayat sudah ada di dalam shell.
#
# Nyalakan dengan ANJURAN_GHOST=1. Bawaannya mati.
# ---------------------------------------------------------------------------

if [[ -n ${ANJURAN_GHOST:-} ]]; then
  if (( $+functions[_zsh_autosuggest_bind_widgets] )) || [[ -n ${ZSH_AUTOSUGGEST_HIGHLIGHT_STYLE:-} ]]; then
    # zsh-autosuggestions sudah melakukan hal yang sama. Memasang keduanya
    # membuat dua teks abu-abu bersaing memperebutkan POSTDISPLAY.
    print -u2 "anjuran: zsh-autosuggestions terdeteksi; saran riwayat anjuran dilewati."
  else

  # Gaya teks bayangan. Warna 8 adalah abu-abu redup di hampir semua tema.
  typeset -g _anjuran_ghost_style=${ANJURAN_GHOST_STYLE:-fg=8}

  _anjuran_ghost_hapus() {
    POSTDISPLAY=""
    region_highlight=()
  }

  _anjuran_ghost_perbarui() {
    _anjuran_ghost_hapus
    (( $#BUFFER )) || return
    # Hanya saat kursor di ujung: bayangan yang muncul di tengah baris
    # menyesatkan, karena ia tidak menyambung apa yang sedang diedit.
    (( CURSOR == $#BUFFER )) || return

    # Karakter pola di dalam buffer dilolos, kalau tidak ia akan dibaca
    # sebagai glob dan mencocokkan hal yang tidak diketik pengguna.
    local prefix="${BUFFER//(#m)[\\()\[\]|*?~^#]/\\$MATCH}"
    local saran="${history[(r)${prefix}*]}"

    [[ -n $saran && $saran != $BUFFER ]] || return
    POSTDISPLAY="${saran#$BUFFER}"
    region_highlight=("$#BUFFER $(( $#BUFFER + $#POSTDISPLAY )) $_anjuran_ghost_style")
  }

  # Widget pengetikan dibungkus supaya bayangan ikut bergerak. Semuanya murni
  # zsh, jadi tidak ada proses yang ditumbuhkan per ketikan.
  _anjuran_ghost_insert() { zle .self-insert; _anjuran_ghost_perbarui }
  _anjuran_ghost_hapus_mundur() { zle .backward-delete-char; _anjuran_ghost_perbarui }
  zle -N self-insert _anjuran_ghost_insert
  zle -N backward-delete-char _anjuran_ghost_hapus_mundur

  # Panah kanan dan End menerima bayangan bila ada; kalau tidak, keduanya
  # kembali menjadi pergerakan kursor biasa.
  _anjuran_ghost_terima() {
    if [[ -n $POSTDISPLAY ]] && (( CURSOR == $#BUFFER )); then
      BUFFER="$BUFFER$POSTDISPLAY"
      CURSOR=$#BUFFER
      _anjuran_ghost_hapus
      return
    fi
    zle .end-of-line
  }
  zle -N _anjuran_ghost_terima
  bindkey "^[[C" _anjuran_ghost_terima
  bindkey "^[OC" _anjuran_ghost_terima
  bindkey "^E" _anjuran_ghost_terima
  [[ -n ${terminfo[kcuf1]} ]] && bindkey "${terminfo[kcuf1]}" _anjuran_ghost_terima

  # Bayangan harus hilang sebelum barisnya dijalankan, kalau tidak teksnya
  # ikut terbaca sebagai bagian perintah oleh mata pengguna.
  _anjuran_ghost_jalankan() { _anjuran_ghost_hapus; zle .accept-line }
  zle -N accept-line _anjuran_ghost_jalankan

  fi
fi
