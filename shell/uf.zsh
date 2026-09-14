# Integrasi uf untuk zsh.
#
# Pasang dengan menambahkan satu baris ini ke ~/.zshrc:
#
#     source /path/ke/uf.zsh
#
# Tombol pemicu bisa diganti lewat UF_KEY sebelum source, misalnya:
#
#     UF_KEY='^ ' source /path/ke/uf.zsh    # Ctrl-Spasi, Tab tetap bawaan zsh

# Jangan pasang apa pun bila binary-nya tidak ada, agar .zshrc tetap aman
# disalin ke mesin yang belum terpasang uf.
(( $+commands[uf] )) || return 0

_uf_widget() {
  emulate -L zsh
  setopt local_options no_ksh_arrays

  local out head body status_word new_cursor

  # Dropdown digambar uf langsung ke /dev/tty; stdout hanya membawa hasil.
  out="$(command uf widget --line "$BUFFER" --cursor "$CURSOR" 2>/dev/null)"

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
