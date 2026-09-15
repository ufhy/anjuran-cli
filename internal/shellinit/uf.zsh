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

_uf_widget() {
  emulate -L zsh
  setopt local_options no_ksh_arrays

  local out head body status_word new_cursor

  # Dropdown digambar uf langsung ke /dev/tty; stdout hanya membawa hasil.
  out="$(command uf widget --line "$BUFFER" --cursor "$CURSOR" \
    --prev-lines "${_uf_lines:-0}" 2>/dev/null)"
  _uf_lines=0

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

  status_word=${head%% *}
  new_cursor=${head##* }

  case $status_word in
    ok)
      BUFFER=$body
      CURSOR=$new_cursor
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
}

zle -N _uf_widget
bindkey "${UF_KEY:-^I}" _uf_widget

# ---------------------------------------------------------------------------
# Dropdown yang muncul sendiri
#
# Pemicunya SPASI, bukan setiap huruf. Setelah sebuah kata selesai diketik,
# barulah ada yang bisa ditawarkan; sebelum itu, isi dropdown hanya akan
# berganti-ganti mengikuti huruf yang belum tentu selesai.
#
# Pilihan itu juga yang membuat fitur ini murah. Satu penggambaran memakan
# sekitar 7 ms karena uf adalah proses baru; dibayar pada setiap huruf, itu
# akan terasa. Dibayar pada spasi — dan pada huruf hanya selagi dropdown sudah
# terbuka — tidak terasa sama sekali.
#
# Nyalakan dengan UF_AUTO=1 sebelum eval. Bawaannya mati, supaya perilaku Tab
# yang sudah ada tidak berubah tanpa diminta.
# ---------------------------------------------------------------------------

if [[ -n ${UF_AUTO:-} ]]; then

  # Jumlah baris yang sedang tergambar. zsh yang menyimpannya, karena setiap
  # pemanggilan uf adalah proses baru yang tidak mewarisi apa pun.
  typeset -g _uf_lines=0

  _uf_draw() {
    _uf_lines=$(command uf render --line "$BUFFER" --cursor "$CURSOR"       --prev-lines "$_uf_lines" 2>/dev/null) || _uf_lines=0
    [[ $_uf_lines == <-> ]] || _uf_lines=0
  }

  _uf_erase() {
    (( _uf_lines )) || return 0
    command uf render --clear --prev-lines "$_uf_lines" >/dev/null 2>&1
    _uf_lines=0
  }

  # self-insert dibungkus, bukan diganti: perilaku aslinya dipanggil lebih dulu
  # lewat .self-insert, sehingga penyisipan karakter tetap milik zsh.
  _uf_insert() {
    zle .self-insert
    # Digambar ulang saat spasi diketik, atau selagi dropdown sudah terbuka
    # supaya isinya ikut menyaring mengikuti ketikan.
    if [[ $KEYS == ' ' ]] || (( _uf_lines )); then
      _uf_draw
    fi
  }
  zle -N self-insert _uf_insert

  # Spasi belum tentu terpasang ke self-insert.
  #
  # oh-my-zsh memetakannya ke magic-space, yang lebih dulu memekarkan rujukan
  # riwayat seperti !! sebelum menyisipkan spasi. Membungkus self-insert saja
  # karena itu tidak pernah terpanggil di sana — dan spasi justru pemicu utama
  # fitur ini. Yang dibungkus harus widget yang SEDANG terpasang, apa pun
  # namanya, supaya perilaku yang sudah dipasang pengguna tetap utuh.
  # Keluarannya berbentuk: " " magic-space
  # Nama widget adalah kata terakhir, jadi segala sesuatu sampai spasi terakhir
  # dibuang. Memecahnya sebagai kata shell tidak bisa dipakai di sini, karena
  # tanda kutip pembungkus tombolnya ikut terhitung sebagai kata tersendiri.
  # Keluarannya berbentuk: " " magic-space
  # Nama widget adalah kata terakhir. Hasilnya ditampung ke variabel lebih dulu;
  # bentuk bersarang seperti ${${(f)"$(...)"}[1]##* } tidak menerapkan
  # pemangkasannya sebagaimana diharapkan.
  typeset -g _uf_space_orig
  typeset _uf_bindkey_out
  _uf_bindkey_out="$(bindkey ' ')"
  _uf_space_orig=${_uf_bindkey_out##* }
  unset _uf_bindkey_out
  if [[ -z $_uf_space_orig || $_uf_space_orig == undefined-key ]]; then
    _uf_space_orig=self-insert
  fi

  _uf_space() {
    if [[ $_uf_space_orig == self-insert ]]; then
      # Widget bawaan dipanggil langsung, supaya tidak berputar kembali ke
      # pembungkus self-insert kita dan menggambar dua kali.
      zle .self-insert
    else
      zle "$_uf_space_orig" || zle .self-insert
    fi
    _uf_draw
  }
  zle -N _uf_space

  bindkey " " _uf_space
  # Mode vi memakai keymap terpisah; tanpa ini fiturnya mati begitu pengguna
  # berpindah ke sana.
  if [[ -n ${keymaps[(r)viins]} ]]; then
    bindkey -M viins " " _uf_space
  fi

  _uf_delete() {
    zle .backward-delete-char
    (( _uf_lines )) && _uf_draw
  }
  zle -N backward-delete-char _uf_delete

  # Dropdown harus hilang sebelum perintahnya dijalankan, kalau tidak
  # keluarannya akan tercetak menimpa kotak yang masih tergambar.
  _uf_accept() { _uf_erase; zle .accept-line }
  zle -N accept-line _uf_accept

  # Ctrl-C tidak pernah sampai ke widget mana pun: ia tiba sebagai SIGINT,
  # bukan sebagai tombol yang dipetakan ZLE. Satu-satunya tempat yang bisa
  # membersihkan sebelum baris dibatalkan adalah trap sinyalnya.
  #
  # Trap milik pengguna tidak ditimpa. Kalau sudah ada, kotak dibiarkan dan
  # disapu oleh precmd pada prompt berikutnya — lebih baik meninggalkan satu
  # kotak daripada mematikan penanganan interupsi yang sudah dipasang orang.
  if ! typeset -f TRAPINT > /dev/null; then
    TRAPINT() {
      _uf_erase
      return $(( 128 + $1 ))
    }
  fi

  # Jaring pengaman: apa pun yang mengakhiri sesi pengeditan ikut membersihkan.
  _uf_line_finish() { _uf_erase }
  zle -N zle-line-finish _uf_line_finish

  # Prompt baru berarti kotak lama sudah tidak relevan. Pencatatannya direset
  # supaya penggambaran berikutnya tidak mencoba menghapus baris yang sudah
  # tergulung keluar layar.
  autoload -Uz add-zsh-hook
  add-zsh-hook precmd _uf_reset
  _uf_reset() { _uf_lines=0 }
fi
