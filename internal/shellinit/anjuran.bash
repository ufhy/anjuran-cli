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

_anjuran_widget() {
  local out head body anjuran_status new_cursor

  # READLINE_POINT dihitung dalam BYTE, berbeda dari zsh dan fish yang
  # menghitung karakter. Salah satuan akan menyisipkan di tempat yang salah
  # begitu baris memuat huruf non-ASCII.
  out="$(command anjuran widget --line "$READLINE_LINE" --cursor "$READLINE_POINT" \
    --cursor-unit byte 2>/dev/null)"

  if [ -z "$out" ]; then
    _anjuran_fallback
    return
  fi

  head=${out%%$'\n'*}
  if [ "$out" != "${out%$'\n'*}" ]; then
    body=${out#*$'\n'}
  else
    body=''
  fi

  anjuran_status=${head%% *}
  new_cursor=${head##* }

  case $anjuran_status in
    ok)
      READLINE_LINE=$body
      READLINE_POINT=$new_cursor
      ;;
    none)
      _anjuran_fallback
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
