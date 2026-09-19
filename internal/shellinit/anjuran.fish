# Integrasi anjuran untuk fish.
#
# Pasang dengan menambahkan satu baris ini ke ~/.config/fish/config.fish:
#
#     anjuran init fish | source
#
# Tombol pemicu bisa diganti lewat ANJURAN_KEY sebelum source, misalnya:
#
#     set -gx ANJURAN_KEY \cspace    # Ctrl-Spasi, Tab tetap milik fish

function _anjuran_widget --description 'Dropdown completion anjuran'
    set -l trigger manual
    if test (count $argv) -gt 0
        set trigger $argv[1]
    end

    set -l select_from first
    if test (count $argv) -gt 1
        set select_from $argv[2]
    end

    set -l line (commandline)
    set -l pos (commandline -C)
    set -l sebelum $line
    set -l alias_exp (_anjuran_alias $line)

    # string collect menjaga baris keluaran tetap utuh; tanpa itu substitusi
    # perintah fish akan memecahnya menjadi daftar dan isi buffer yang memuat
    # newline ikut hancur.
    set -l out (command anjuran widget --line "$line" --cursor $pos --trigger $trigger --select $select_from --alias "$alias_exp" 2>/dev/null | string collect)

    # Keluaran kosong berarti anjuran mati di tengah jalan. Menyerahkannya ke
    # completion fish hanya benar bila pengguna memang MEMINTA penyisipan,
    # yaitu saat ia menekan Tab. Pada pemicu otomatis, tidak ada yang muncul.
    if test -z "$out"
        if test $trigger = manual
            commandline -f complete
        end
        return
    end

    set -l parts (string split -m 1 \n -- $out)
    set -l head $parts[1]
    set -l body ''
    if test (count $parts) -gt 1
        set body $parts[2]
    end

    set -l fields (string split ' ' -- $head)
    set -l anjuran_status $fields[1]
    set -l new_cursor $fields[2]
    set -l sisa ''
    if test (count $fields) -gt 2
        set sisa $fields[3]
    end

    switch $anjuran_status
        case ok
            commandline -r -- $body
            commandline -C $new_cursor

            # Memilih sebuah direktori membuka isinya. Garis miring adalah
            # karakter pemicu, sama seperti spasi — bahwa ia datang dari
            # pilihan alih-alih diketik tidak mengubah apa pun.
            #
            # Dibuka TANPA ada yang tersorot, supaya Enter berarti "cukup,
            # pakai path ini" dan penelusuran punya cara berhenti. Kutip
            # penutup dilepas dulu: nama berspasi disisipkan terkutip.
            set -l ekor (string trim -r -c '\'"' -- $body)
            if test "$body" != "$sebelum" -a -z "$sisa"
                if string match -q '*/' -- $ekor
                    _anjuran_widget auto none
                end
            end
        case none
            # Tidak ada spec untuk perintah ini. Completion bawaan fish sudah
            # sangat baik, jadi serahkan kembali kepadanya — tetapi hanya bila
            # pengguna yang memintanya.
            if test $trigger = manual
                commandline -f complete
            end
        case '*'
            # Dibatalkan: biarkan baris apa adanya.
    end
end

function _anjuran_bind --description 'Pasang tombol pemicu anjuran'
    set -l key \t
    if set -q ANJURAN_KEY
        set key $ANJURAN_KEY
    end

    bind $key _anjuran_widget
    # fish memisahkan mode default dan insert saat binding vi aktif; tanpa
    # baris ini tombolnya mati begitu pengguna memakai mode vi.
    # Dicoba langsung, tanpa bertanya lebih dulu.
    #
    # Sebelumnya dukungannya diperiksa dengan `bind --help`, dan fish 4
    # menjawabnya dengan MENCETAK SELURUH HALAMAN MANUAL ke terminal — setiap
    # kali shell dibuka. Kegagalan `bind -M` sendiri tidak berbahaya: ia
    # hanya berarti mode vi tidak tersedia, dan galatnya sudah dibungkam.
    bind -M insert $key _anjuran_widget 2>/dev/null

    # Pemicu otomatis: kotak muncul begitu sebuah kata selesai ditulis, bukan
    # sambil mengetik. Sama persis dengan yang dipasang integrasi zsh dan bash
    # — pemicu yang berbeda antar shell membuat alat yang sama terasa seperti
    # dua alat berbeda begitu seseorang berpindah mesin.
    #
    # NYALA secara bawaan. Matikan dengan ANJURAN_AUTO=0.
    if test "$ANJURAN_AUTO" = 0 -o "$ANJURAN_AUTO" = no -o "$ANJURAN_AUTO" = off -o "$ANJURAN_AUTO" = false
        return
    end
    bind ' ' _anjuran_spasi
    bind / _anjuran_garismiring
    bind = _anjuran_samadengan
    bind -M insert ' ' _anjuran_spasi 2>/dev/null
    bind -M insert / _anjuran_garismiring 2>/dev/null
    bind -M insert = _anjuran_samadengan 2>/dev/null
end

# _anjuran_alias mencari arti alias untuk kata pertama segmen TERAKHIR.
#
# Alias hanya berlaku di posisi perintah, jadi "docker ps | gst" memakai gst,
# bukan docker. Di fish sebuah alias adalah fungsi, jadi yang dibaca badan
# fungsinya — bukan sebuah tabel seperti di zsh dan bash.
function _anjuran_alias
    set -l kata (string split ' ' -- $argv[1])
    set -l pertama ''
    for w in $kata
        switch $w
            case '|' '||' '&&' ';' '&'
                set pertama ''
            case ''
                # lewati
            case '*'
                if test -z "$pertama"
                    set pertama $w
                end
        end
    end
    test -n "$pertama"; or return

    functions -q $pertama; or return
    # Badan fungsi alias fish berbentuk "command git status $argv"; yang
    # dibutuhkan hanya perintahnya, tanpa pembungkus dan tanpa $argv.
    set -l badan (functions $pertama 2>/dev/null \
        | string match -r '^\s+(?:command )?(\S.*)$' \
        | string replace -r ' \$argv.*$' '' )
    test (count $badan) -ge 2; or return
    echo $badan[2]
end

# _anjuran_pemicu menyisipkan karakternya sendiri, lalu membuka sesi.
#
# Menyisipkan sendiri adalah keharusan, bukan pilihan: bind mengambil alih
# tombolnya sepenuhnya, jadi tanpa ini karakter yang diketik pengguna hilang.
# Tidak ada penjagaan tempelan di sini, dan itu bukan kelalaian.
#
# zsh memakai PENDING + KEYS_QUEUED_COUNT, bash memakai `read -t 0`, dan
# PowerShell memakai [Console]::KeyAvailable — ketiganya bisa menanyakan
# "apakah masih ada ketikan yang antre". fish tidak menyediakan pertanyaan itu
# kepada skrip sama sekali.
#
# Yang menjaganya di fish adalah bracketed paste: terminal membungkus tempelan
# dengan ESC[200~ dan ESC[201~, dan fish membaca seluruh blok itu lalu
# menyisipkannya sekaligus TANPA menjalankan binding per karakter. Semua
# terminal yang masih dirawat mendukungnya.
#
# Yang tersisa: terminal tanpa bracketed paste. Di sana tempelan akan membuka
# satu kotak per spasi, dan satu-satunya jalan keluar adalah ANJURAN_AUTO=0.
function _anjuran_pemicu
    commandline -i -- $argv[1]

    # Karakternya digemakan sendiri sebelum kotak digambar.
    #
    # fish baru menggambar ulang barisnya SESUDAH fungsi binding ini selesai,
    # dan saat itu anjuran sudah terlanjur menggambar kotaknya. Tanpa ini,
    # mengetik "cd " menampilkan "cd" dengan daftar direktori di bawahnya —
    # spasinya tidak pernah terlihat, walau buffer-nya sendiri benar.
    #
    # Hanya saat kursor berada di UJUNG baris. Di tengah baris, menyisipkan
    # sebuah karakter berarti menggeser teks di kanannya, dan menggemakannya
    # begitu saja justru menimpa yang sudah ada.
    set -l isi (commandline)
    if test (commandline -C) -eq (string length -- "$isi")
        printf '%s' $argv[1] > /dev/tty 2>/dev/null
    end

    _anjuran_widget auto
end

function _anjuran_spasi
    _anjuran_pemicu ' '
end

function _anjuran_garismiring
    _anjuran_pemicu /
end

function _anjuran_samadengan
    _anjuran_pemicu =
end

_anjuran_bind

# ---------------------------------------------------------------------------
# Saran dari riwayat (ghost text)
#
# Teks abu-abu yang melanjutkan ketikan berdasarkan perintah yang pernah
# dijalankan. Di zsh fitur ini ditulis sendiri, karena zsh tidak punya
# padanannya. fish SUDAH punya, bawaan, dan miliknya lebih baik: ia memakai
# riwayat beserta konteks direktori dan diperbarui oleh shell sendiri tanpa
# proses tambahan.
#
# Jadi yang benar bukan menirunya di sini — dua teks abu-abu bertumpuk hanya
# menambah kebisingan — melainkan memakai milik fish, dengan saklar yang sama
# seperti shell lain: ANJURAN_GHOST.
if test "$ANJURAN_GHOST" = 1 -o "$ANJURAN_GHOST" = yes -o "$ANJURAN_GHOST" = on -o "$ANJURAN_GHOST" = true
    set -g fish_autosuggestion_enabled 1
end
