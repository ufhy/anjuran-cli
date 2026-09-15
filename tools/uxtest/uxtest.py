#!/usr/bin/env python3
"""Uji UX uf di dalam zsh sungguhan.

Menjalankan skenario yang benar-benar diketik orang, lalu memeriksa apa yang
TERLIHAT di layar. Uji Go memeriksa jawaban engine; berkas ini memeriksa
pengalamannya — apakah kotaknya muncul, apakah isinya benar, apakah ia hilang
saat seharusnya hilang.

Dijalankan dengan konfigurasi shell ASLI pengguna, bukan zsh kosong: zsh kosong
menyembunyikan seluruh kelas bug, karena di sana spasi terpasang ke self-insert
sementara oh-my-zsh memetakannya ke magic-space.

    make ux                 jalankan semuanya
    make ux SKENARIO=alias  jalankan yang namanya memuat "alias"
"""

import os
import shutil
import subprocess
import sys
import tempfile

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import term  # noqa: E402

REPO = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
ZSH = "/bin/zsh"


class Hasil:
    def __init__(self):
        self.lulus = 0
        self.gagal = []


def siapkan_pemasangan():
    """Susun binary beserta spec-nya seperti hasil pemasangan sungguhan.

    uf mencari spec relatif terhadap dirinya sendiri. Menjalankan bin/uf apa
    adanya berarti tidak ada spec sama sekali — dan seluruh skenario akan gagal
    karena alasan yang tidak ada hubungannya dengan UX. Menyusunnya seperti
    paket rilis sekaligus menguji tata letak itu.
    """
    d = tempfile.mkdtemp(prefix="uf-bin-")
    shutil.copy2(os.path.join(REPO, "bin", "uf"), os.path.join(d, "uf"))
    for nama in ("specs", "extra"):
        asal = os.path.join(REPO, nama)
        if os.path.isdir(asal):
            shutil.copytree(asal, os.path.join(d, nama))
    return d


def siapkan_sandbox():
    """Direktori sekali pakai berisi berkas yang bisa dilengkapi.

    TIDAK PERNAH di dalam repo: sesi shell sungguhan menjalankan perintah
    sungguhan, dan Enter pada kandidat tunggal mengeksekusi barisnya.
    """
    d = tempfile.mkdtemp(prefix="uf-ux-")
    os.makedirs(os.path.join(d, "berkas"), exist_ok=True)
    os.makedirs(os.path.join(d, "berkas-lain"), exist_ok=True)
    for f in ["README.md", "catatan.txt", "data.json", "berkas dengan spasi.txt"]:
        open(os.path.join(d, f), "w").close()
    # Direktori yang BERISI, supaya menelusuri ke dalamnya punya sesuatu untuk
    # ditampilkan.
    for f in ["dalam-satu.txt", "dalam-dua.txt"]:
        open(os.path.join(d, "berkas", f), "w").close()
    subprocess.run(["git", "init", "-q", d], check=False)
    env = {"GIT_DIR": os.path.join(d, ".git"), "GIT_WORK_TREE": d}
    e = dict(os.environ, **env)
    subprocess.run(["git", "-C", d, "commit", "-q", "--allow-empty", "-m", "awal"],
                   check=False, env=e, capture_output=True)
    for b in ["fitur-alpha", "fitur-beta"]:
        subprocess.run(["git", "-C", d, "branch", b], check=False, env=e, capture_output=True)
    return d


def jalankan(nama, ketikan, periksa, sandbox, bindir, auto=True):
    """Jalankan satu skenario; periksa(teks_layar) mengembalikan None atau alasan gagal."""
    env = {"PATH": bindir + ":" + os.environ["PATH"]}
    s = term.Sesi([ZSH, "-i", "-l"], cwd=sandbox, env=env)
    try:
        s.tunggu(3.0)
        s.ketik('PROMPT="%% "\r', 0.6)
        baris = 'UF_AUTO=1 eval "$(uf init zsh)"' if auto else 'eval "$(uf init zsh)"'
        s.ketik(baris + "\r", 1.5)
        s.bersihkan_layar()
        for k in ketikan:
            s.ketik(k)
        return periksa(s.layar.teks())
    finally:
        s.tutup()


def memuat(*bagian):
    def f(teks):
        for b in bagian:
            if b not in teks:
                return f"tidak memuat {b!r}\n--- layar ---\n{teks[-400:]}"
        return None
    return f


def tanpa(*bagian):
    def f(teks):
        for b in bagian:
            if b in teks:
                return f"seharusnya tidak memuat {b!r}\n--- layar ---\n{teks[-400:]}"
        return None
    return f


def gabung(*fns):
    def f(teks):
        for fn in fns:
            r = fn(teks)
            if r:
                return r
        return None
    return f


SKENARIO = [
    # (nama, ketikan, pemeriksa)
    ("spasi memunculkan kotak", [b"git", b" "], memuat("╭", "commit")),
    ("mengetik menyaring", [b"git", b" ", b"co"], gabung(memuat("commit"), tanpa("archive"))),
    ("kandidat tunggal tetap tampil", [b"git", b" ", b"stat"], memuat("status")),
    ("cocok tanpa peduli huruf besar", [b"cat", b" ", b"rea"], memuat("README.md")),
    ("panah membuka mode memilih", [b"git", b" ", b"\x1b[B"], memuat("❯")),
    # Bingkai kotak tidak bisa dipakai sebagai penanda: prompt powerlevel10k
    # memakai karakter yang sama. Yang diperiksa adalah isinya.
    ("Esc menutup kotak", [b"git", b" ", b"\x1b[B", b"\x1b"], tanpa("commit", "archive")),
    # Nama berspasi hanya bisa diketik terkutip atau terlolos; tanpa itu shell
    # sudah memecahnya menjadi dua kata sebelum uf melihatnya.
    ("berkas berspasi terkutip", [b"cat", b" ", b'"berkas d'], memuat("berkas dengan spasi.txt")),
    ("berkas berspasi terlolos", [b"cat", b" ", b"berkas\\ d"], memuat("berkas dengan spasi.txt")),
    ("direktori berakhir garis miring", [b"cd", b" ", b"berk"], memuat("berkas/")),
    ("menelusuri direktori", [b"ls", b" ", b"berkas/"], memuat("dalam-satu.txt")),
    ("branch git sungguhan", [b"git", b" ", b"checkout", b" "], memuat("fitur-alpha")),
    ("Tab menyisipkan awalan bersama", [b"git", b" ", b"checkout", b" ", b"fit", b"\t"],
     memuat("fitur-")),
    ("perintah tanpa spec melengkapi berkas", [b"gzip", b" "], memuat("README.md")),
    ("perintah asing melengkapi berkas", [b"perintahkarangan", b" "], memuat("README.md")),
    # Bingkai tidak bisa dipakai sebagai penanda: prompt powerlevel10k memakai
    # karakter yang sama. Yang diperiksa adalah isinya.
    ("tanpa kandidat tidak ada kotak", [b"git", b" ", b"zzzq"], tanpa("commit", "checkout")),
    ("Tab tetap jalan tanpa mode otomatis", [b"git", b" ", b"ch", b"\t"], memuat("checkout")),
]


def main():
    saring = os.environ.get("SKENARIO", "")
    if not os.path.exists(os.path.join(REPO, "bin", "uf")):
        print("bin/uf belum ada; jalankan `make build` lebih dulu", file=sys.stderr)
        return 2

    bindir = siapkan_pemasangan()
    sandbox = siapkan_sandbox()
    print(f"pemasangan: {bindir}\nsandbox   : {sandbox}\n")

    h = Hasil()
    for nama, ketikan, periksa in SKENARIO:
        if saring and saring not in nama:
            continue
        auto = "tanpa mode otomatis" not in nama
        alasan = jalankan(nama, ketikan, periksa, sandbox, bindir, auto=auto)
        if alasan is None:
            h.lulus += 1
            print(f"  lulus  {nama}")
        else:
            h.gagal.append((nama, alasan))
            print(f"  GAGAL  {nama}")

    print(f"\n{h.lulus} lulus, {len(h.gagal)} gagal")
    for nama, alasan in h.gagal:
        print(f"\n=== {nama} ===\n{alasan}")
    return 1 if h.gagal else 0


if __name__ == "__main__":
    sys.exit(main())
