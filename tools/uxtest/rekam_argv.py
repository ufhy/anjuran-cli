#!/usr/bin/env python3
"""Rekam argumen persis yang dikirim shell ke anjuran pada SATU skenario.

Dipakai saat sebuah skenario gagal di satu mesin dan tidak di mesin lain.
Layar yang direkam hanya memberi tahu APA yang terlihat; berkas ini memberi
tahu apa yang benar-benar diminta shell — dan selisih di antara keduanya
biasanya justru cacatnya.

    SHELL_UJI=bash python3 tools/uxtest/rekam_argv.py "kubectl| |get| |pods| |--output|="

Ketikan dipisah dengan garis tegak. Tanpa argumen, skenario kubectl dipakai.
"""

import os
import shutil
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import uxtest  # noqa: E402

LOG = "/tmp/argv.log"


def pemasangan_berpembungkus():
    """Pemasangan biasa, dengan anjuran diganti pembungkus yang mencatat argv."""
    d = uxtest.siapkan_pemasangan()
    asli = os.path.join(d, "anjuran-asli")
    shutil.move(os.path.join(d, "anjuran"), asli)
    with open(os.path.join(d, "anjuran"), "w") as f:
        f.write("#!/bin/sh\n")
        f.write('printf "%s\\n" "$*" >> ' + LOG + "\n")
        f.write('exec "$(dirname "$0")/anjuran-asli" "$@"\n')
    os.chmod(os.path.join(d, "anjuran"), 0o755)
    return d


def main():
    mentah = sys.argv[1] if len(sys.argv) > 1 else "kubectl| |get| |pods| |--output|="
    ketikan = [b.encode() for b in mentah.split("|")]
    shell = os.environ.get("SHELL_UJI", "zsh")

    d = pemasangan_berpembungkus()
    sandbox = uxtest.siapkan_sandbox()
    open(LOG, "w").close()

    alasan = uxtest.jalankan("rekam", ketikan, uxtest.memuat("json"),
                             sandbox, d, "/tmp/cache-rekam", shell=shell)
    print("shell :", shell)
    print("hasil :", "lulus" if alasan is None else alasan)
    print("=== argv yang dikirim shell ===")
    with open(LOG) as f:
        print(f.read())


if __name__ == "__main__":
    main()
