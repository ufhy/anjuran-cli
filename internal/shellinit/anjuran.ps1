# Integrasi anjuran untuk PowerShell.
#
# Pasang dengan menambahkan satu baris ini ke $PROFILE:
#
#     anjuran init powershell | Out-String | Invoke-Expression
#
# Membutuhkan PSReadLine, yang sudah menjadi bawaan PowerShell 5.1 ke atas.
# Tombol pemicu bisa diganti lewat $env:ANJURAN_KEY sebelum dijalankan, misalnya:
#
#     $env:ANJURAN_KEY = 'Ctrl+Spacebar'   # Tab tetap milik PowerShell

if (-not (Get-Module -ListAvailable -Name PSReadLine)) {
    Write-Warning 'anjuran: PSReadLine tidak tersedia; integrasi dilewati.'
    return
}
Import-Module PSReadLine -ErrorAction SilentlyContinue

$script:UfKey = if ($env:ANJURAN_KEY) { $env:ANJURAN_KEY } else { 'Tab' }

# Set-AnjuranKey memasang satu tombol untuk mode edit apa pun.
#
# PSReadLine menyimpan tabel tombol terpisah untuk mode vi. Pengguna yang
# menulis `Set-PSReadLineOption -EditMode Vi` di profilnya — biasanya sesudah
# memuat integrasi ini — akan mendapati seluruh tombol anjuran tidak ada di
# sana. Gejalanya bukan fitur yang berkurang melainkan anjuran yang mati
# total, tanpa satu pun pesan.
#
# -ViMode baru ada di PSReadLine 2.x; kegagalannya ditelan supaya versi lama
# tetap mendapat binding biasa.
function Set-AnjuranKey {
    param([string]$Key, [scriptblock]$Aksi, [string]$Keterangan)

    Set-PSReadLineKeyHandler -Key $Key -BriefDescription $Keterangan -ScriptBlock $Aksi

    # -ViMode hanya dipakai bila mode vi MEMANG sedang aktif. PSReadLine
    # menolak mendaftarkan tombol untuk mode yang belum dipakai, dan
    # memanggilnya tetap hanya menghasilkan peringatan di setiap shell start
    # tanpa memasang apa pun.
    #
    # Akibatnya urutan di profil menjadi penting, dan itu disebut di README:
    # `Set-PSReadLineOption -EditMode Vi` harus berada SEBELUM baris anjuran.
    # Bila dibalik, seluruh tombol anjuran hilang tanpa satu pun pesan.
    if ((Get-PSReadLineOption).EditMode -eq 'Vi') {
        try {
            Set-PSReadLineKeyHandler -Key $Key -BriefDescription $Keterangan -ScriptBlock $Aksi -ViMode Insert -ErrorAction Stop
        } catch {
            Write-Verbose 'anjuran: PSReadLine tanpa -ViMode; mode vi dilewati.'
        }
    }
}

# Get-AnjuranAlias mencari arti alias untuk kata pertama segmen TERAKHIR.
#
# Alias hanya berlaku di posisi perintah, jadi "docker ps | gst" memakai gst,
# bukan docker.
function Get-AnjuranAlias {
    param([string]$Line)

    $pertama = ''
    foreach ($w in ($Line -split '\s+')) {
        if ($w -in @('|', '||', '&&', ';', '&')) { $pertama = ''; continue }
        if ($w -and -not $pertama) { $pertama = $w }
    }
    if (-not $pertama) { return '' }

    $a = Get-Alias -Name $pertama -ErrorAction SilentlyContinue
    if ($a) { return $a.Definition }
    return ''
}

function Invoke-AnjuranWidget {
    param([string]$Trigger = 'manual', [string]$Select = 'first')

    $line = $null
    $cursor = $null
    [Microsoft.PowerShell.PSConsoleReadLine]::GetBufferState([ref]$line, [ref]$cursor)

    # Indeks PSReadLine adalah indeks string .NET, yaitu UTF-16 code unit.
    # Bukan byte seperti bash, bukan pula rune seperti zsh dan fish; pada emoji
    # ketiganya berbeda sekaligus.
    $sebelum = $line
    $aliasExp = Get-AnjuranAlias -Line $line

    $out = @()
    try {
        $out = @(& anjuran widget --line $line --cursor $cursor --cursor-unit utf16 --trigger $Trigger --select $Select --alias $aliasExp 2>$null)
    } catch {
        # Binary tidak ditemukan atau gagal dijalankan: jangan matikan tombolnya.
    }

    # Keluaran kosong berarti anjuran mati di tengah jalan. Menyisipkan sesuatu
    # sebagai gantinya hanya benar bila pengguna memang MEMINTA penyisipan,
    # yaitu saat ia menekan Tab. Pada pemicu otomatis, tidak ada yang muncul.
    if ($out.Count -eq 0) {
        if ($Trigger -eq 'manual') {
            [Microsoft.PowerShell.PSConsoleReadLine]::MenuComplete()
        }
        return
    }

    $head = $out[0]
    $body = if ($out.Count -gt 1) { ($out[1..($out.Count - 1)] -join "`n") } else { '' }

    $fields = $head -split ' '
    $ufStatus = $fields[0]
    $newCursor = 0
    [void][int]::TryParse($fields[1], [ref]$newCursor)
    $sisa = if ($fields.Count -gt 2) { $fields[2] } else { '' }

    switch ($ufStatus) {
        'ok' {
            # Replace mengganti seluruh buffer sekaligus, sehingga PSReadLine
            # menggambar ulang satu kali saja alih-alih per karakter.
            [Microsoft.PowerShell.PSConsoleReadLine]::Replace(0, $line.Length, $body)
            [Microsoft.PowerShell.PSConsoleReadLine]::SetCursorPosition($newCursor)

            # Memilih sebuah direktori membuka isinya. Garis miring adalah
            # karakter pemicu, sama seperti spasi — bahwa ia datang dari
            # pilihan alih-alih diketik tidak mengubah apa pun.
            #
            # Dibuka TANPA ada yang tersorot, supaya Enter berarti "cukup,
            # pakai path ini" dan penelusuran punya cara berhenti. Kutip
            # penutup dilepas dulu: nama berspasi disisipkan terkutip.
            $ekor = $body.TrimEnd("'", '"')
            if ($body -ne $sebelum -and -not $sisa -and ($ekor.EndsWith('/') -or $ekor.EndsWith('\'))) {
                Invoke-AnjuranWidget -Trigger $Trigger -Select 'none'
            }
        }
        'none' {
            # Tidak ada spec untuk perintah ini. Completion bawaan PowerShell
            # mengenal cmdlet, parameter, dan berkas, jadi serahkan kepadanya —
            # tetapi hanya bila pengguna yang memintanya.
            if ($Trigger -eq 'manual') {
                [Microsoft.PowerShell.PSConsoleReadLine]::MenuComplete()
            }
        }
        default {
            # Dibatalkan: biarkan baris apa adanya.
        }
    }
}

Set-AnjuranKey -Key $script:UfKey -Keterangan 'anjuran' -Aksi {
    Invoke-AnjuranWidget -Trigger 'manual'
}

# ---------------------------------------------------------------------------
# Pemicu otomatis
#
# Kotak muncul begitu sebuah kata selesai ditulis, bukan sambil mengetik:
# spasi menutup sebuah kata, / menutup sebuah komponen path, = menutup nama
# sebuah opsi. Sama persis dengan zsh, bash, dan fish — pemicu yang berbeda
# antar shell membuat alat yang sama terasa seperti dua alat berbeda begitu
# seseorang berpindah mesin.
#
# NYALA secara bawaan. Matikan dengan $env:ANJURAN_AUTO = '0'.

if ($env:ANJURAN_AUTO -notin @('0', 'no', 'off', 'false')) {
    # Backslash ikut memicu, dan hanya di sini. Path Windows memakai "\",
    # sehingga tanpa itu satu-satunya pemicu path di shell ini tidak pernah
    # ditekan siapa pun: mengetik "cd C:\" tidak memunculkan apa-apa.
    foreach ($karakter in @(' ', '/', '\', '=')) {
        # Karakternya disisipkan sendiri: handler mengambil alih tombolnya
        # sepenuhnya, jadi tanpa Insert karakter yang diketik pengguna hilang.
        $chord = if ($karakter -eq ' ') { 'Spacebar' } else { $karakter }
        # Kutip TUNGGAL dengan sengaja: di dalamnya PowerShell tidak memproses
        # backslash sebagai escape, sehingga '\' sampai apa adanya.
        Set-AnjuranKey -Key $chord -Keterangan 'anjuran-auto' -Aksi ([scriptblock]::Create(@"
            [Microsoft.PowerShell.PSConsoleReadLine]::Insert('$karakter')
            Invoke-AnjuranWidget -Trigger 'auto'
"@))
    }
}

# ---------------------------------------------------------------------------
# Saran dari riwayat (ghost text)
#
# Teks abu-abu yang melanjutkan ketikan berdasarkan perintah yang pernah
# dijalankan. Di zsh fitur ini ditulis sendiri, karena zsh tidak punya
# padanannya. PSReadLine SUDAH punya lewat PredictionSource, dan miliknya
# lebih baik: ia bisa memakai plugin prediksi selain riwayat, dan
# diperbarui oleh PSReadLine sendiri tanpa proses tambahan.
#
# Jadi yang dipakai milik PSReadLine, dengan saklar yang sama seperti shell
# lain: ANJURAN_GHOST. PredictionSource baru ada di PSReadLine 2.1, jadi
# kegagalannya ditelan — shell yang lebih tua tetap jalan tanpa fitur ini.
if ($env:ANJURAN_GHOST -in @('1', 'yes', 'on', 'true')) {
    try {
        Set-PSReadLineOption -PredictionSource History -ErrorAction Stop
    } catch {
        Write-Verbose 'anjuran: PSReadLine terlalu tua untuk PredictionSource; ghost text dilewati.'
    }
}
