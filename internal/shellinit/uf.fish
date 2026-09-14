# Integrasi uf untuk fish.
#
# Pasang dengan menambahkan satu baris ini ke ~/.config/fish/config.fish:
#
#     uf init fish | source
#
# Tombol pemicu bisa diganti lewat UF_KEY sebelum source, misalnya:
#
#     set -gx UF_KEY \cspace    # Ctrl-Spasi, Tab tetap milik fish

function _uf_widget --description 'Dropdown completion uf'
    set -l line (commandline)
    set -l pos (commandline -C)

    # string collect menjaga baris keluaran tetap utuh; tanpa itu substitusi
    # perintah fish akan memecahnya menjadi daftar dan isi buffer yang memuat
    # newline ikut hancur.
    set -l out (command uf widget --line "$line" --cursor $pos 2>/dev/null | string collect)

    if test -z "$out"
        commandline -f complete
        return
    end

    set -l parts (string split -m 1 \n -- $out)
    set -l head $parts[1]
    set -l body ''
    if test (count $parts) -gt 1
        set body $parts[2]
    end

    set -l fields (string split ' ' -- $head)
    set -l uf_status $fields[1]
    set -l new_cursor $fields[2]

    switch $uf_status
        case ok
            commandline -r -- $body
            commandline -C $new_cursor
        case none
            # Tidak ada spec untuk perintah ini. Completion bawaan fish sudah
            # sangat baik, jadi serahkan kembali kepadanya.
            commandline -f complete
        case '*'
            # Dibatalkan: biarkan baris apa adanya.
    end
end

function _uf_bind --description 'Pasang tombol pemicu uf'
    set -l key \t
    if set -q UF_KEY
        set key $UF_KEY
    end

    bind $key _uf_widget
    # fish memisahkan mode default dan insert saat binding vi aktif; tanpa
    # baris ini tombolnya mati begitu pengguna memakai mode vi.
    if bind --help 2>/dev/null | string match -q '*-M*'
        bind -M insert $key _uf_widget 2>/dev/null
    end
end

_uf_bind
