# Pemasang anjuran untuk Windows.
#
#   irm https://raw.githubusercontent.com/ufhy/anjuran-cli/main/install.ps1 | iex
#
# TIDAK pernah meminta hak administrator. Semuanya dipasang di bawah
# %LOCALAPPDATA%, dan PATH yang diubah adalah PATH pengguna — bukan PATH
# sistem, yang memang bukan milik sebuah pemasang untuk disentuh.
#
# Lingkungan yang dibaca:
#   ANJURAN_VERSION     tag rilis tertentu, misalnya v0.1.0 (bawaan: terbaru)
#   ANJURAN_INSTALL_DIR direktori dasar (bawaan: $env:LOCALAPPDATA\anjuran)
#   ANJURAN_NO_SHELL    bila diisi, jangan sentuh profil PowerShell
#   ANJURAN_DRY_RUN     bila diisi, laporkan rencananya tanpa mengunduh apa pun
#   ANJURAN_BASE_URL    asal berkas rilis; untuk cermin, dan supaya skrip ini
#                       bisa diuji tanpa menerbitkan rilis sungguhan

$ErrorActionPreference = 'Stop'

$Repo = 'ufhy/anjuran-cli'
$Asal = if ($env:ANJURAN_BASE_URL) { $env:ANJURAN_BASE_URL } else { "https://github.com/$Repo/releases/download" }
$Base = if ($env:ANJURAN_INSTALL_DIR) { $env:ANJURAN_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'anjuran' }

# Penanda yang sama dipakai install.sh dan `anjuran up`, supaya blok yang
# ditulis salah satunya dikenali oleh yang lain dan tidak pernah ditumpuk.
$PenandaAwal = '# >>> anjuran >>>'
$PenandaAkhir = '# <<< anjuran <<<'

function Get-AnjuranArch {
    # PROCESSOR_ARCHITECTURE melaporkan arsitektur PROSES, dan PowerShell 32-bit
    # di Windows 64-bit akan menyebut x86. Yang benar ditanyakan ke .NET.
    switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
        'X64'   { 'amd64' }
        'Arm64' { 'arm64' }
        default {
            throw "arsitektur $_ belum didukung"
        }
    }
}

function Get-AnjuranVersiTerbaru {
    $r = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
    if (-not $r.tag_name) { throw 'tidak bisa membaca rilis terbaru dari GitHub; sebutkan ANJURAN_VERSION' }
    return $r.tag_name
}

$arch = Get-AnjuranArch
$versi = if ($env:ANJURAN_VERSION) { $env:ANJURAN_VERSION } else { Get-AnjuranVersiTerbaru }

# Nama berkas rilis memakai versi TANPA awalan v, sedangkan tagnya memakainya.
$polos = $versi -replace '^v', ''
$nama = "anjuran_${polos}_windows_${arch}.zip"
$url = "$Asal/$versi/$nama"

Write-Host 'Memasang anjuran:'
Write-Host ''
Write-Host "  versi    : $versi"
Write-Host "  platform : windows/$arch"
Write-Host "  dari     : $url"
Write-Host "  ke       : $Base\anjuran.exe"
Write-Host ''

if ($env:ANJURAN_DRY_RUN) {
    Write-Host 'Mode dry-run; tidak ada yang diunduh.'
    return
}

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("anjuran-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
try {
    $arsip = Join-Path $tmp $nama
    Invoke-WebRequest -Uri $url -OutFile $arsip -UseBasicParsing

    # Arsip diunduh lewat jaringan, dan yang dipasangnya adalah biner yang
    # akan dijalankan setiap kali pengguna menekan Tab. Memeriksanya bukan
    # kemewahan.
    $daftar = Join-Path $tmp 'checksums.txt'
    try {
        Invoke-WebRequest -Uri "$Asal/$versi/checksums.txt" -OutFile $daftar -UseBasicParsing
    } catch {
        $daftar = $null
        Write-Host '  checksum : dilewati (checksums.txt tidak ada di rilis ini)'
    }
    if ($daftar) {
        $baris = Get-Content $daftar | Where-Object { $_ -match "\s\*?$([regex]::Escape($nama))$" } | Select-Object -First 1
        if (-not $baris) { throw "$nama tidak ada di checksums.txt" }
        $mau = ($baris -split '\s+')[0]
        $dapat = (Get-FileHash -Algorithm SHA256 -Path $arsip).Hash.ToLower()
        if ($dapat -ne $mau.ToLower()) {
            throw "checksum tidak cocok untuk $nama`n  mau   : $mau`n  dapat : $dapat"
        }
        Write-Host '  checksum : cocok'
    }

    $bongkar = Join-Path $tmp 'isi'
    Expand-Archive -Path $arsip -DestinationPath $bongkar -Force
    $exe = Join-Path $bongkar 'anjuran.exe'
    if (-not (Test-Path $exe)) { throw 'arsipnya tidak memuat anjuran.exe' }

    New-Item -ItemType Directory -Force -Path $Base | Out-Null
    Copy-Item $exe (Join-Path $Base 'anjuran.exe') -Force

    # Spec lama dibuang lebih dulu agar berkas yang sudah tidak ada di rilis
    # baru tidak tertinggal dan tetap ditawarkan.
    foreach ($d in @('specs', 'extra')) {
        $asal = Join-Path $bongkar $d
        if (Test-Path $asal) {
            $tujuan = Join-Path $Base $d
            if (Test-Path $tujuan) { Remove-Item $tujuan -Recurse -Force }
            Copy-Item $asal $tujuan -Recurse -Force
        }
    }

    $terpasang = & (Join-Path $Base 'anjuran.exe') version
    Write-Host "  terpasang: $terpasang"
    $jumlah = @(Get-ChildItem -Path (Join-Path $Base 'specs') -Filter '*.json.gz' -Recurse -ErrorAction SilentlyContinue).Count
    Write-Host "  spec     : $jumlah"

    # PATH PENGGUNA, bukan PATH sistem: yang kedua butuh hak administrator dan
    # berlaku untuk semua orang di mesin itu — keputusan yang bukan milik
    # sebuah pemasang untuk diambil.
    $pathPengguna = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($pathPengguna -notlike "*$Base*") {
        [Environment]::SetEnvironmentVariable('Path', "$Base;$pathPengguna", 'User')
        Write-Host "  PATH     : $Base ditambahkan ke PATH pengguna"
    } else {
        Write-Host '  PATH     : sudah ada'
    }
    $env:Path = "$Base;$env:Path"

    if (-not $env:ANJURAN_NO_SHELL) {
        $profil = $PROFILE.CurrentUserAllHosts
        New-Item -ItemType Directory -Force -Path (Split-Path $profil) | Out-Null
        if ((Test-Path $profil) -and (Select-String -Path $profil -SimpleMatch $PenandaAwal -Quiet)) {
            Write-Host "  shell    : $profil (sudah ada)"
        } else {
            Add-Content -Path $profil -Value @(
                '',
                $PenandaAwal,
                'anjuran init powershell | Out-String | Invoke-Expression',
                $PenandaAkhir
            )
            Write-Host "  shell    : $profil (ditambahkan)"
        }
    }

    Write-Host ''
    Write-Host 'Buka sesi PowerShell baru, lalu tekan spasi sesudah sebuah perintah.'
} finally {
    Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}
