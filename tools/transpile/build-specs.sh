#!/bin/sh
# Bangun direktori spec uf dari paket npm @withfig/autocomplete (MIT).
#
# Paket itu sudah berisi spec terkompilasi sebagai modul ESM, sehingga yang
# dibutuhkan hanya Node — bukan TypeScript, bukan pula meng-clone reponya.
#
# Pemakaian: build-specs.sh <versi-fig> <dir-keluaran>

set -eu

VERSION="${1:?versi fig tidak diberikan}"
OUT="${2:?direktori keluaran tidak diberikan}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

URL="https://registry.npmjs.org/@withfig/autocomplete/-/autocomplete-${VERSION}.tgz"

printf 'mengunduh  : %s\n' "$URL"
curl -fsSL --retry 3 -o "$WORK/specs.tgz" "$URL"

printf 'membongkar : %s\n' "$WORK"
tar xzf "$WORK/specs.tgz" -C "$WORK"

printf 'transpile  : -> %s\n\n' "$OUT"
rm -rf "$OUT"
node "$(dirname "$0")/transpile.mjs" "$WORK/package/build" "$OUT"

# Spec disimpan ter-gzip. Seluruh spec Fig berukuran 45 MB sebagai JSON polos
# dan sekitar 7 MB ter-gzip; selisih itu terasa saat spec ikut dikirim ke host
# remote, dan biaya membuka satu berkas kecil jauh di bawah anggaran waktu kita.
printf '\nmengompresi...\n'
find "$OUT" -name '*.json' -print0 | xargs -0 -P 8 gzip -9

# Lisensi sumber ikut dibawa karena isi direktori ini memang karya Fig.
cp "$WORK/package/LICENSE" "$OUT/LICENSE" 2>/dev/null || true
cat > "$OUT/README.md" <<README
# specs

Direktori ini DIHASILKAN, jangan disunting langsung. Jalankan \`make specs\`
untuk membangunnya ulang.

Sumber : @withfig/autocomplete versi ${VERSION} (MIT, lihat LICENSE)
Bentuk : skema spec Fig yang sudah disaring, disimpan sebagai <nama>.json.gz

Yang dibuang saat transpile: generator berbentuk fungsi dan \`custom\`,
\`postProcess\`, \`generateSpec\`, \`parserDirectives\`, dan ikon. Semuanya
membutuhkan mesin JavaScript saat runtime. Generator berbentuk argv tetap
dibawa, sehingga tidak ada perintah yang perlu dilewatkan ke shell.

Deskripsi dipotong menjadi satu baris pendek; dropdown memang tidak pernah
menampilkan lebih dari itu.
README

printf '\nselesai: %s\n' "$(du -sh "$OUT" | cut -f1)"
