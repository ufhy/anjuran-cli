#!/bin/sh
# Menguji `anjuran bootstrap` lewat koneksi SSH SUNGGUHAN.
#
# Jalur remote adalah bagian yang paling jarang tersentuh, dan uji Go-nya
# memakai sh lokal dengan $HOME dialihkan — jaringannya tidak pernah ikut,
# sehingga "apa yang benar-benar mendarat di sana" tidak pernah diperiksa dari
# sisi penerima. Justru di situlah cacat pertama ditemukan: extra/ tidak ikut
# terkirim, dan seluruh tambalan buatan tangan hilang di host remote.
#
# Yang dilakukan: menyalakan sshd SEKALI PAKAI di port tinggi sebagai pengguna
# biasa, mem-bootstrap ke sana, memeriksa hasilnya, lalu membereskan semuanya.
#
# Tidak perlu sudo. Tidak menyentuh konfigurasi mesin. Remote Login macOS TIDAK
# dinyalakan — itu mengekspos mesin ke jaringan, sedangkan ini hanya mendengar
# di 127.0.0.1 dengan autentikasi kunci.
set -eu

REPO=$(cd "$(dirname "$0")/../.." && pwd)
PORT=${PORT:-22022}
BASE=.anjuran-sshuji          # dipisahkan dari pemasangan sungguhan pengguna
T=$(mktemp -d /tmp/anjuran-sshuji.XXXXXX)
SRC=""

bersihkan() {
  [ -f "$T/sshd/pid" ] && kill "$(cat "$T/sshd/pid")" 2>/dev/null || true
  rm -rf "$HOME/$BASE" "$HOME/$BASE-proyek" "$T" ${SRC:+"$SRC"}
}
trap bersihkan EXIT INT TERM

gagal() { printf '  GAGAL  %s\n' "$1"; exit 1; }
lulus() { printf '  lulus  %s\n' "$1"; }

# --- sshd sekali pakai ------------------------------------------------------
mkdir -p "$T/home/.ssh" "$T/sshd"
chmod 700 "$T/home/.ssh"
ssh-keygen -q -t ed25519 -f "$T/sshd/host_key" -N '' -C sshuji
ssh-keygen -q -t ed25519 -f "$T/home/.ssh/id" -N '' -C sshuji
cp "$T/home/.ssh/id.pub" "$T/sshd/authorized_keys"
chmod 600 "$T/sshd/authorized_keys"

cat > "$T/sshd/sshd_config" <<CFG
Port $PORT
ListenAddress 127.0.0.1
HostKey $T/sshd/host_key
AuthorizedKeysFile $T/sshd/authorized_keys
PidFile $T/sshd/pid
StrictModes no
UsePAM no
PasswordAuthentication no
PubkeyAuthentication yes
LogLevel QUIET
CFG

# UserKnownHostsFile /dev/null: known_hosts milik pengguna tidak boleh
# ketambahan entri dari host yang hanya hidup beberapa detik.
cat > "$T/ssh_config" <<CFG
Host sshuji
  HostName 127.0.0.1
  Port $PORT
  User $(id -un)
  IdentityFile $T/home/.ssh/id
  IdentitiesOnly yes
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null
CFG

/usr/sbin/sshd -f "$T/sshd/sshd_config" 2>"$T/sshd/err" || {
  echo "sshd gagal dijalankan:"; cat "$T/sshd/err"; exit 1; }
sleep 1

SSH="ssh -F $T/ssh_config -o LogLevel=ERROR sshuji"
$SSH 'echo ok' >/dev/null 2>&1 || gagal "tidak bisa tersambung ke sshd uji"
lulus "tersambung ke sshd sekali pakai di 127.0.0.1:$PORT"

# --- sumber yang akan dikirim ----------------------------------------------
[ -x "$REPO/bin/anjuran" ] || gagal "bin/anjuran belum ada; jalankan \`make build\`"
SRC=$(mktemp -d)
cp "$REPO/bin/anjuran" "$SRC/"
cp -R "$REPO/specs" "$SRC/" 2>/dev/null || true
cp -R "$REPO/extra" "$SRC/"

# --- bootstrap --------------------------------------------------------------
"$REPO/bin/anjuran" bootstrap --yes --force --from "$SRC" --base "$BASE" \
  sshuji -F "$T/ssh_config" -o LogLevel=ERROR > "$T/keluaran" 2>&1 \
  || { cat "$T/keluaran"; gagal "bootstrap"; }
grep -q terpasang "$T/keluaran" || { cat "$T/keluaran"; gagal "bootstrap tidak melaporkan terpasang"; }
lulus "bootstrap selesai ($(awk '/terkirim/{print $3, $4}' "$T/keluaran"))"

BIN="\$HOME/$BASE/bin/anjuran"
$SSH "test -x $BIN" || gagal "binary tidak mendarat di $BASE/bin/anjuran"
lulus "binary mendarat dan bisa dijalankan"

# --- yang membedakan dari uji lokal: apa yang benar-benar ada di sana --------
$SSH "$BIN complete --line 'git comm' --cursor 8" | grep -q '^commit' \
  || gagal "spec tidak terbaca di host remote"
lulus "spec terbaca di host remote"

# extra/ adalah tambalan buatan tangan; ia pernah tidak ikut terkirim sama
# sekali, dan completion di host remote diam-diam lebih buruk karenanya.
$SSH "mkdir -p \$HOME/$BASE-proyek && printf '{\"scripts\":{\"terbang\":\"kubectl apply -f k8s/\"}}' > \$HOME/$BASE-proyek/package.json"
$SSH "cd \$HOME/$BASE-proyek && $BIN complete --line 'bun run ' --cursor 9" | grep -q '^terbang' \
  || gagal "tambalan extra/ tidak sampai: 'bun run' tidak membaca package.json di sana"
lulus "extra/ sampai — 'bun run' membaca package.json MILIK HOST itu"

printf '\n5 lulus, 0 gagal\n'
