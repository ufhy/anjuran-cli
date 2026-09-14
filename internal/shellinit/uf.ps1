# Integrasi uf untuk PowerShell.
#
# Pasang dengan menambahkan satu baris ini ke $PROFILE:
#
#     uf init powershell | Out-String | Invoke-Expression
#
# Membutuhkan PSReadLine, yang sudah menjadi bawaan PowerShell 5.1 ke atas.
# Tombol pemicu bisa diganti lewat $env:UF_KEY sebelum dijalankan, misalnya:
#
#     $env:UF_KEY = 'Ctrl+Spacebar'   # Tab tetap milik PowerShell

if (-not (Get-Module -ListAvailable -Name PSReadLine)) {
    Write-Warning 'uf: PSReadLine tidak tersedia; integrasi dilewati.'
    return
}
Import-Module PSReadLine -ErrorAction SilentlyContinue

$script:UfKey = if ($env:UF_KEY) { $env:UF_KEY } else { 'Tab' }

Set-PSReadLineKeyHandler -Key $script:UfKey -BriefDescription 'uf' -LongDescription 'Dropdown completion uf' -ScriptBlock {
    $line = $null
    $cursor = $null
    [Microsoft.PowerShell.PSConsoleReadLine]::GetBufferState([ref]$line, [ref]$cursor)

    # Indeks PSReadLine adalah indeks string .NET, yaitu UTF-16 code unit.
    # Bukan byte seperti bash, bukan pula rune seperti zsh dan fish; pada emoji
    # ketiganya berbeda sekaligus.
    $out = @()
    try {
        $out = @(& uf widget --line $line --cursor $cursor --cursor-unit utf16 2>$null)
    } catch {
        # Binary tidak ditemukan atau gagal dijalankan: jangan matikan tombolnya.
    }

    if ($out.Count -eq 0) {
        [Microsoft.PowerShell.PSConsoleReadLine]::MenuComplete()
        return
    }

    $head = $out[0]
    $body = if ($out.Count -gt 1) { ($out[1..($out.Count - 1)] -join "`n") } else { '' }

    $fields = $head -split ' '
    $ufStatus = $fields[0]
    $newCursor = 0
    [void][int]::TryParse($fields[1], [ref]$newCursor)

    switch ($ufStatus) {
        'ok' {
            # Replace mengganti seluruh buffer sekaligus, sehingga PSReadLine
            # menggambar ulang satu kali saja alih-alih per karakter.
            [Microsoft.PowerShell.PSConsoleReadLine]::Replace(0, $line.Length, $body)
            [Microsoft.PowerShell.PSConsoleReadLine]::SetCursorPosition($newCursor)
        }
        'none' {
            # Tidak ada spec untuk perintah ini. Completion bawaan PowerShell
            # mengenal cmdlet, parameter, dan berkas, jadi serahkan kepadanya.
            [Microsoft.PowerShell.PSConsoleReadLine]::MenuComplete()
        }
        default {
            # Dibatalkan: biarkan baris apa adanya.
        }
    }
}
