# Integrasi uf untuk zsh.
#
# Pasang dengan menambahkan satu baris ini ke ~/.zshrc:
#
#     eval "$(uf init zsh)"
#
# Tombol pemicu bisa diganti lewat UF_KEY sebelum eval, misalnya:
#
#     UF_KEY='^ ' eval "$(uf init zsh)"     # Ctrl-Spasi, Tab tetap bawaan zsh

# Jangan pasang apa pun bila binary-nya tidak ada, agar .zshrc tetap aman
# disalin ke mesin yang belum terpasang uf.
(( $+commands[uf] )) || return 0

# _uf_alias menaruh pemekaran alias kata pertama ke dalam _uf_alias_exp.
#
# Tanpa ini alias sama sekali tidak dikenali: uf mencari spec bernama "gco" dan
# tidak menemukannya. Padahal alias justru cara sehari-hari orang memakai
# perintah panjang, dan oh-my-zsh memasang ratusan di antaranya.
#
# Nilainya diletakkan di variabel, bukan dicetak, supaya tidak menumbuhkan
# subshell pada jalur yang dijalankan setiap kali spasi ditekan.
typeset -g _uf_alias_exp

_uf_alias() {
  emulate -L zsh
  setopt local_options no_ksh_arrays

  _uf_alias_exp=""

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
  _uf_alias_exp=${aliases[$first]-}
}

# _uf_widget menjalankan SATU sesi yang memegang seluruh interaksi.
#
# Model ini mengikuti cara IDE bekerja: daftar kandidat diambil sekali, lalu
# disaring di tempat sambil pengguna mengetik — bukan dihitung ulang dari nol
# pada setiap ketikan. Satu proses per interaksi, bukan satu proses per huruf.
#
# Konsekuensinya sesi memegang masukan selama dropdown terbuka. Agar tidak ada
# yang dirampas dari zsh, tombol yang bukan urusan dropdown dikembalikan lewat
# zle -U dan diproses zsh seperti biasa.
_uf_widget() {
  emulate -L zsh
  setopt local_options no_ksh_arrays

  local out head body status_word new_cursor sisa
  local select_from=${1:-first}
  # Pemicu otomatis tidak pernah berarti "sisipkan yang ini"; hanya Tab yang
  # berarti begitu.
  local trigger=${2:-manual}

  # Bayangan disembunyikan selama dropdown terbuka: dua saran sekaligus hanya
  # menambah kebisingan, dan teks setelah kursor mengganggu gambar kotaknya.
  if (( $+functions[_uf_ghost_hapus] )); then
    _uf_ghost_hapus
  fi

  # Baris digambar ulang LEBIH DULU.
  #
  # Karakter pemicu baru saja disisipkan ke buffer, tetapi zsh belum
  # menampilkannya — ia menggambar setelah widget selesai. Tanpa ini uf mulai
  # menggambar dari kolom yang salah, dan karakter pemicunya tidak pernah
  # terlihat.
  zle redisplay

  _uf_alias
  # Dropdown digambar uf langsung ke /dev/tty; stdout hanya membawa hasil.
  out="$(command uf widget --line "$BUFFER" --cursor "$CURSOR" \
    --select "$select_from" --trigger "$trigger" \
    --alias "$_uf_alias_exp" 2>/dev/null)"

  if [[ -z $out ]]; then
    # uf tidak bisa menjalankan sesi, misalnya karena bukan terminal
    # interaktif. Serahkan ke completion bawaan zsh.
    zle expand-or-complete
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
      # Tab yang menghasilkan sebuah direktori membuka isinya. Hanya untuk
      # pemicu manual: Tab memang berarti "lengkapi lagi", sedangkan pemicu
      # otomatis tidak boleh terus membuka kotak tanpa diminta.
      if [[ $trigger == manual && $BUFFER == */ && -z $sisa ]]; then
        zle redisplay
        _uf_widget first manual
        return
      fi
      ;;
    none)
      # Tidak ada spec untuk perintah ini. Completion bawaan zsh masih jauh
      # lebih baik daripada tidak ada apa-apa.
      zle expand-or-complete
      return
      ;;
    cancel|*)
      # Biarkan buffer apa adanya.
      ;;
  esac

  zle redisplay
  _uf_kembalikan "$sisa"

  # Bayangan dihitung ulang untuk baris yang baru.
  if (( $+functions[_uf_ghost_perbarui] )); then
    _uf_ghost_perbarui
  fi
}

# _uf_kembalikan mengembalikan tombol yang belum ditangani ke antrean masukan
# zsh, sehingga diproses seperti tidak pernah lewat uf.
#
# Dikirim sebagai heksadesimal karena isinya byte kendali yang tidak aman
# dilewatkan apa adanya di dalam satu baris teks.
_uf_kembalikan() {
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

zle -N _uf_widget

# ---------------------------------------------------------------------------
# Pemicu
#
# Tab selalu membuka sesi. Dengan UF_AUTO, karakter pemicu ikut membukanya —
# mengikuti cara IDE: bukan satu tombol khusus, melainkan titik-titik di mana
# ada sesuatu yang layak ditawarkan.
# ---------------------------------------------------------------------------

bindkey "${UF_KEY:-^I}" _uf_widget

if [[ -n ${UF_AUTO:-} ]]; then
  # Widget asli yang terpasang pada sebuah tombol, supaya perilakunya tetap
  # utuh sebelum sesi dibuka. oh-my-zsh memetakan spasi ke magic-space, yang
  # memekarkan rujukan riwayat lebih dulu.
  typeset -gA _uf_asli

  _uf_simpan_asli() {
    local key=$1 nama=$2 keluaran orig
    keluaran="$(bindkey "$key")"
    orig=${keluaran##* }
    if [[ -n $orig && $orig != undefined-key ]]; then
      _uf_asli[$nama]=$orig
    fi
  }

  # _uf_pemicu menjalankan widget asli tombolnya, lalu membuka sesi.
  _uf_pemicu() {
    local nama=$1
    local orig=${_uf_asli[$nama]}
    if [[ -n $orig && $orig != $nama ]]; then
      zle "$orig" 2>/dev/null || zle .self-insert
    else
      zle .self-insert
    fi
    _uf_widget first auto
  }

  _uf_spasi()      { _uf_pemicu _uf_spasi }
  _uf_garismiring(){ _uf_pemicu _uf_garismiring }
  _uf_samadengan() { _uf_pemicu _uf_samadengan }
  zle -N _uf_spasi
  zle -N _uf_garismiring
  zle -N _uf_samadengan

  _uf_simpan_asli " " _uf_spasi
  _uf_simpan_asli "/" _uf_garismiring
  _uf_simpan_asli "=" _uf_samadengan

  bindkey " " _uf_spasi
  bindkey "/" _uf_garismiring
  bindkey "=" _uf_samadengan
  if [[ -n ${keymaps[(r)viins]} ]]; then
    bindkey -M viins " " _uf_spasi
    bindkey -M viins "/" _uf_garismiring
    bindkey -M viins "=" _uf_samadengan
  fi
fi

# ---------------------------------------------------------------------------
# Saran dari riwayat (ghost text)
#
# Teks abu-abu yang melanjutkan ketikan berdasarkan perintah yang pernah
# dijalankan. Untuk perintah panjang yang diulang setiap hari — kubectl logs
# dengan namespace dan selector — ini lebih sering menolong daripada dropdown.
#
# Seluruhnya dikerjakan di dalam zsh, tanpa memanggil uf sama sekali: ia harus
# diperbarui pada SETIAP ketikan, dan menumbuhkan proses di sana akan terasa
# berat. uf hanya menggambar dropdown; riwayat sudah ada di dalam shell.
#
# Nyalakan dengan UF_GHOST=1. Bawaannya mati.
# ---------------------------------------------------------------------------

if [[ -n ${UF_GHOST:-} ]]; then
  if (( $+functions[_zsh_autosuggest_bind_widgets] )) || [[ -n ${ZSH_AUTOSUGGEST_HIGHLIGHT_STYLE:-} ]]; then
    # zsh-autosuggestions sudah melakukan hal yang sama. Memasang keduanya
    # membuat dua teks abu-abu bersaing memperebutkan POSTDISPLAY.
    print -u2 "uf: zsh-autosuggestions terdeteksi; saran riwayat uf dilewati."
  else

  # Gaya teks bayangan. Warna 8 adalah abu-abu redup di hampir semua tema.
  typeset -g _uf_ghost_style=${UF_GHOST_STYLE:-fg=8}

  _uf_ghost_hapus() {
    POSTDISPLAY=""
    region_highlight=()
  }

  _uf_ghost_perbarui() {
    _uf_ghost_hapus
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
    region_highlight=("$#BUFFER $(( $#BUFFER + $#POSTDISPLAY )) $_uf_ghost_style")
  }

  # Widget pengetikan dibungkus supaya bayangan ikut bergerak. Semuanya murni
  # zsh, jadi tidak ada proses yang ditumbuhkan per ketikan.
  _uf_ghost_insert() { zle .self-insert; _uf_ghost_perbarui }
  _uf_ghost_hapus_mundur() { zle .backward-delete-char; _uf_ghost_perbarui }
  zle -N self-insert _uf_ghost_insert
  zle -N backward-delete-char _uf_ghost_hapus_mundur

  # Panah kanan dan End menerima bayangan bila ada; kalau tidak, keduanya
  # kembali menjadi pergerakan kursor biasa.
  _uf_ghost_terima() {
    if [[ -n $POSTDISPLAY ]] && (( CURSOR == $#BUFFER )); then
      BUFFER="$BUFFER$POSTDISPLAY"
      CURSOR=$#BUFFER
      _uf_ghost_hapus
      return
    fi
    zle .end-of-line
  }
  zle -N _uf_ghost_terima
  bindkey "^[[C" _uf_ghost_terima
  bindkey "^[OC" _uf_ghost_terima
  bindkey "^E" _uf_ghost_terima
  [[ -n ${terminfo[kcuf1]} ]] && bindkey "${terminfo[kcuf1]}" _uf_ghost_terima

  # Bayangan harus hilang sebelum barisnya dijalankan, kalau tidak teksnya
  # ikut terbaca sebagai bagian perintah oleh mata pengguna.
  _uf_ghost_jalankan() { _uf_ghost_hapus; zle .accept-line }
  zle -N accept-line _uf_ghost_jalankan

  fi
fi
