#!/usr/bin/env python3
"""Uji UX anjuran di dalam zsh sungguhan.

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

    anjuran mencari spec relatif terhadap dirinya sendiri. Menjalankan bin/anjuran apa
    adanya berarti tidak ada spec sama sekali — dan seluruh skenario akan gagal
    karena alasan yang tidak ada hubungannya dengan UX. Menyusunnya seperti
    paket rilis sekaligus menguji tata letak itu.
    """
    d = tempfile.mkdtemp(prefix="anjuran-bin-")
    shutil.copy2(os.path.join(REPO, "bin", "anjuran"), os.path.join(d, "anjuran"))
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
    d = tempfile.mkdtemp(prefix="anjuran-ux-")
    os.makedirs(os.path.join(d, "berkas"), exist_ok=True)
    os.makedirs(os.path.join(d, "berkas-lain"), exist_ok=True)
    # Bertingkat, supaya menelusuri punya tempat untuk TERUS turun — itulah
    # yang dulu terjadi tanpa diminta.
    os.makedirs(os.path.join(d, "proyek", "dalam", "lebih"), exist_ok=True)
    # Folder BERSPASI: namanya terpecah jadi dua kata oleh shell sebelum anjuran
    # melihatnya, dan disisipkan terkutip sesudahnya.
    os.makedirs(os.path.join(d, "folder dengan spasi", "dalam sini"), exist_ok=True)
    open(os.path.join(d, "folder dengan spasi", "isi.txt"), "w").close()
    # Cukup banyak direktori supaya kotaknya setinggi layar pada terminal
    # pendek — di sanalah ruang layarnya pernah habis. Namanya sengaja
    # mengurut paling belakang agar urutan skenario lain tidak berubah.
    for i in range(1, 7):
        os.makedirs(os.path.join(d, f"zz-{i}"), exist_ok=True)
    # Berkas proyek: perintah yang didefinisikan pengguna sendiri, yang tidak
    # mungkin diketahui spec mana pun.
    with open(os.path.join(d, "package.json"), "w") as f:
        f.write('{"scripts":{"dev":"vite --port 3000","bangun":"tsc && vite build"}}')
    with open(os.path.join(d, "Makefile"), "w") as f:
        f.write("pasang: ## pasang dependensi\n\tnpm ci\n\nbersihkan:\n\trm -rf bin\n")
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


def jalankan(nama, ketikan, periksa, sandbox, bindir, cachedir, auto=True, ghost=False,
             persiapan=(), rows=24):
    """Jalankan satu skenario; periksa(teks_layar) mengembalikan None atau alasan gagal."""
    env = {
        "PATH": bindir + ":" + os.environ["PATH"],
        # Cache diarahkan ke direktori sekali pakai supaya ingatan pilihan
        # milik pengguna tidak ikut berubah saat pengujian.
        "ANJURAN_CACHE_DIR": cachedir,
    }
    s = term.Sesi([ZSH, "-i", "-l"], cwd=sandbox, env=env, rows=rows)
    try:
        s.tunggu(3.0)
        s.ketik('PROMPT="%% "\r', 0.6)
        env_awal = []
        if auto is True:
            env_awal.append("ANJURAN_AUTO=1")
        elif auto is False:
            env_awal.append("ANJURAN_AUTO=0")
        if ghost:
            env_awal.append("ANJURAN_GHOST=1")
        s.ketik(" ".join(env_awal) + ' eval "$(anjuran init zsh)"\r', 1.5)
        for baris in persiapan:
            s.ketik(baris + "\r", 0.8)
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


# Baris yang dijalankan sebelum skenario, untuk mengisi riwayat perintah.
PERSIAPAN = {
    "bayangan melanjutkan dari riwayat": (
        "echo kubectl logs -f pod-produksi --namespace produksi",
    ),
    "bayangan diterima panah kanan": (
        "echo kubectl logs -f pod-produksi --namespace produksi",
    ),
    "bayangan hilang saat dropdown terbuka": (
        "echo git checkout fitur-alpha",
    ),
}

SKENARIO = [
    # (nama, ketikan, pemeriksa)
    # Nama perintah yang sudah lengkap langsung menawarkan ISINYA, termasuk
    # perintah yang isinya berupa argumen dan bukan subcommand.
    ("mengetik cd menawarkan direktori", [b"c", b"d"], memuat("proyek/")),
    ("cd yang dipilih benar-benar mendarat",
     [b"c", b"d", b"\x1b[B", b"\r", b"\r", b"pwd\r"], memuat("/berkas")),
    # Tab pada baris KOSONG menawarkan perintah. Dijalankan tanpa mode otomatis
    # supaya yang membuka kotak benar-benar Tab, bukan ketikan sesudahnya.
    ("Tab pada baris kosong tanpa mode otomatis",
     [b"\t", b"zs"], memuat("zsh")),
    # Di terminal pendek, kotaknya pernah menuntut ruang tepat sebanyak tinggi
    # layar: baris perintah terdorong keluar saat menggulung, dan prompt
    # beserta awal perintahnya lenyap. Buffer-nya benar — perintahnya tetap
    # berjalan — sehingga hanya tampilannya yang bohong.
    ("terminal pendek tetap menampilkan barisnya",
     [b"c", b"d", b"\r"], memuat("cd berkas-lain/")),

    # Teks yang disisipkan SESI juga harus terlihat, bukan hanya yang diketik.
    # Shell tidak menggambar ulang selama widget-nya berjalan, jadi Tab yang
    # menyisipkan "yek/" meninggalkan layar menampilkan "cd pro" — benar
    # isinya, bohong tampilannya.
    ("Tab menelusuri terlihat di baris",
     [b"cd", b" ", b"pro", b"\t"], memuat("cd proyek/")),
    ("panah kanan masuk folder terlihat di baris",
     [b"cd", b" ", b"pro", b"\x1b[C"], memuat("cd proyek/")),
    ("hapus sesudah menelusuri terlihat di baris",
     [b"cd", b" ", b"pro", b"\t", b"\x7f", b"\x7f", b"\x7f"],
     memuat("cd proy")),
    # Menghapus perintah sampai habis harus MENUTUP kotaknya.
    #
    # Dibiarkan terbuka, yang ditawarkan adalah seluruh isi PATH dengan
    # kandidat pertama menurut abjad tersorot — sesuatu yang tidak pernah
    # diketik siapa pun. Enter berikutnya menyisipkannya, dan Enter sesudahnya
    # MENJALANKANNYA.
    # Diperiksa lewat perintah BERIKUTNYA, bukan lewat isi kotaknya: nama
    # perintah mana yang tersorot bergantung pada isi PATH mesinnya, sedangkan
    # akibatnya tidak. Bila kotak dibiarkan terbuka, Enter menyisipkan sebuah
    # nama perintah, dan "echo" yang diketik sesudahnya menempel di belakangnya
    # menjadi satu kata yang tidak ada — "command not found".
    # Tab pada baris kosong menampilkan seluruh isi PATH, tetapi TIDAK
    # menyorot apa pun: menekan Tab lalu Enter di sana pernah cukup untuk
    # menjalankan biner asing — nama pertama menurut abjad.
    ("Tab pada baris kosong tidak menyisipkan tanpa diminta",
     [b"\t", b"\r", b"echo TANDA\r"],
     gabung(memuat("TANDA"), tanpa("command not found"))),
    ("hapus sampai habis lalu Enter tidak menyisipkan apa pun",
     [b"c", b"d", b"\x7f", b"\x7f", b"\r", b"echo TANDA\r"],
     gabung(memuat("TANDA"), tanpa("command not found"))),

    # Perintah yang didefinisikan pengguna di berkas proyek. Spec Fig
    # membacanya lewat `bash -c`, yang ditolak kebijakan generator; anjuran
    # membaca berkasnya sendiri.
    ("skrip package.json ditawarkan",
     [b"bun", b" ", b"run", b" "], memuat("dev", "bangun")),
    ("keterangan skrip adalah isi perintahnya",
     [b"npm", b" ", b"run", b" "], memuat("vite --port 3000")),
    ("target Makefile ditawarkan",
     [b"make", b" "], memuat("pasang", "bersihkan")),
    ("keterangan target diambil dari '##'",
     [b"make", b" "], memuat("pasang dependensi")),

    # Yang diketik harus TERLIHAT. Seluruh skenario lain memeriksa isi
    # kotaknya, sehingga satu huruf yang hilang dari baris masukan tidak pernah
    # ketahuan — padahal buffer-nya benar dan perintahnya tetap jalan.
    ("ketikan terlihat saat kotak terbuka", [b"g", b"i", b"t"], memuat("git")),
    ("ketikan terlihat pada pemicu spasi",
     [b"git", b" ", b"co"], memuat("git co")),
    # Mengetik nama perintah sudah cukup; tidak perlu spasi maupun Tab.
    ("mengetik nama perintah memunculkan isinya",
     [b"g", b"i", b"t"], memuat("checkout", "commit")),
    ("nama perintah dilengkapi dari PATH",
     [b"k", b"u", b"b", b"e", b"c"], memuat("kubectl")),
    # Bawaannya nyala: tanpa menyetel apa pun, kotaknya tetap muncul.
    ("pemicu otomatis nyala tanpa disetel",
     [b"git", b" "], memuat("commit")),
    ("spasi memunculkan kotak", [b"git", b" "], memuat("╭", "commit")),
    ("mengetik menyaring", [b"git", b" ", b"co"], gabung(memuat("commit"), tanpa("archive"))),
    ("kandidat tunggal tetap tampil", [b"git", b" ", b"stat"], memuat("status")),
    ("cocok tanpa peduli huruf besar", [b"cat", b" ", b"rea"], memuat("README.md")),
    ("panah membuka mode memilih", [b"git", b" ", b"\x1b[B"], memuat("❯")),
    # Bingkai kotak tidak bisa dipakai sebagai penanda: prompt powerlevel10k
    # memakai karakter yang sama. Yang diperiksa adalah isinya.
    ("Esc menutup kotak", [b"git", b" ", b"\x1b[B", b"\x1b"], tanpa("commit", "archive")),
    # Nama berspasi hanya bisa diketik terkutip atau terlolos; tanpa itu shell
    # sudah memecahnya menjadi dua kata sebelum anjuran melihatnya.
    ("berkas berspasi terkutip", [b"cat", b" ", b'"berkas d'], memuat("berkas dengan spasi.txt")),
    ("berkas berspasi terlolos", [b"cat", b" ", b"berkas\\ d"], memuat("berkas dengan spasi.txt")),
    ("direktori berakhir garis miring", [b"cd", b" ", b"berk"], memuat("berkas/")),
    ("menelusuri direktori", [b"ls", b" ", b"berkas/"], memuat("dalam-satu.txt")),
    # Folder berspasi harus berperilaku sama persis dengan folder biasa:
    # ditelusuri, diberi ikon, dan disisipkan terkutip.
    ("folder berspasi tanpa kutip tetap muncul",
     [b"cd", b" ", b"folder de"],
     memuat("folder dengan spasi/")),
    ("folder berspasi dapat ikon tombol",
     [b"cd", b" ", b"folder de"],
     memuat("\u2192 \u23ce")),
    # Diperiksa lewat tempat mendaratnya, bukan tampilan: bila pengutipannya
    # salah, cd memecah namanya jadi dua argumen dan gagal sama sekali.
    ("Tab pada folder berspasi mengutipnya",
     [b"cd", b" ", b"folder de", b"\t", b"\x1b", b"\r", b"pwd\r"],
     gabung(memuat("/folder dengan spasi"),
            tanpa("/folder dengan spasi/dalam sini"))),
    ("folder berspasi bisa ditelusuri",
     [b"cd", b" ", b"folder de", b"\t", b"\t", b"\r", b"\r", b"pwd\r"],
     memuat("/folder dengan spasi/dalam sini")),
    ("dua argumen terpisah tidak ikut disatukan",
     [b"ls", b" ", b"berkas cat"],
     gabung(memuat("catatan.txt"), tanpa("berkas/"))),

    # Folder punya dua tindakan; ikon di tepi kanan baris terpilih memberi
    # tahu keduanya ada, dan panah kanan benar-benar melakukannya.
    ("baris folder menampilkan ikon tombol",
     [b"cd", b" "],
     memuat("\u2192 \u23ce")),
    ("ikon tidak muncul pada kandidat biasa",
     [b"git", b" ", b"comm"],
     gabung(memuat("commit"), tanpa("\u23ce"))),
    ("panah kanan masuk ke folder",
     [b"cd", b" ", b"pro", b"\x1b[B", b"\x1b[C", b"\r", b"\r", b"pwd\r"],
     gabung(memuat("/proyek"), tanpa("/proyek/dalam"))),

    # Menelusuri direktori dulu menyeret turun tanpa henti: isinya dibuka
    # dengan anak pertama tersorot, dan bila anaknya tunggal ia disisipkan lalu
    # ditelusuri lagi, sampai dasar. Sekarang isinya ditampilkan tanpa ada yang
    # dipilihkan, sehingga Enter berarti "cukup, pakai path ini".
    ("isi direktori tampil tanpa dipilihkan",
     [b"cd", b" ", b"pro", b"\t"],
     gabung(memuat("proyek/dalam/"), tanpa("\u276f proyek/dalam/"))),
    ("Enter berhenti di direktori yang sudah dipilih",
     [b"cd", b" ", b"pro", b"\t", b"\r", b"\r", b"pwd\r"],
     gabung(memuat("/proyek"), tanpa("/proyek/dalam"))),
    ("Tab turun satu tingkat tiap tekan",
     [b"cd", b" ", b"pro", b"\t", b"\t", b"\r", b"\r", b"pwd\r"],
     gabung(memuat("/proyek/dalam"), tanpa("/proyek/dalam/lebih"))),
    ("branch git sungguhan", [b"git", b" ", b"checkout", b" "], memuat("fitur-alpha")),
    ("Tab menyisipkan awalan bersama", [b"git", b" ", b"checkout", b" ", b"fit", b"\t"],
     memuat("fitur-")),
    ("perintah tanpa spec melengkapi berkas", [b"gzip", b" "], memuat("README.md")),
    ("perintah asing melengkapi berkas", [b"perintahkarangan", b" "], memuat("README.md")),
    # Bingkai tidak bisa dipakai sebagai penanda: prompt powerlevel10k memakai
    # karakter yang sama. Yang diperiksa adalah isinya.
    ("tanpa kandidat tidak ada kotak", [b"git", b" ", b"zzzq"], tanpa("commit", "checkout")),
    ("Tab tetap jalan tanpa mode otomatis", [b"git", b" ", b"ch", b"\t"], memuat("checkout")),

    # Yang pernah dipilih tersorot lebih dulu di kali berikutnya, alih-alih
    # pengguna menekan panah ke entri yang sama setiap hari.
    ("yang pernah dipilih tersorot duluan",
     [b"git", b" ", b"che", b"\x1b[B", b"\x1b[B", b"\r",   # pilih kandidat kedua
      b"\x15",                                             # Ctrl-U: bersihkan baris
      b"git", b" ", b"che", b"\t"],                        # ulangi awalan yang sama
     memuat("❯ cherry-pick")),

    # Teks abu-abu yang melanjutkan ketikan dari perintah yang pernah
    # dijalankan. Untuk perintah panjang yang diulang setiap hari, ini lebih
    # sering menolong daripada dropdown.
    ("bayangan melanjutkan dari riwayat",
     [b"echo kubectl log"],
     memuat("--namespace produksi")),

    ("bayangan diterima panah kanan",
     [b"echo kubectl log", b"\x1b[C"],
     memuat("echo kubectl logs -f pod-produksi --namespace produksi")),

    # Dua saran sekaligus hanya menambah kebisingan, dan teks setelah kursor
    # mengganggu gambar kotaknya.
    # Ketikan cepat tiba dalam satu bongkahan. Karakter yang menyusul setelah
    # sesi menutup harus tetap sampai ke buffer — hilangnya terasa sebagai
    # karakter yang kadang tidak muncul.
    ("ketikan cepat tidak ada yang hilang",
     [b"echo satu", b"dua", b"tiga"],
     memuat("echo satuduatiga")),

    ("ketikan cepat sesudah pemicu",
     [b"echo", b" ", b"kubectl"],
     memuat("echo kubectl")),

    # Enter harus MENERIMA lalu menutup, kalau tidak perintahnya tidak pernah
    # bisa dijalankan: setiap Enter hanya turun satu tingkat lagi.
    ("cd bisa dijalankan",
     [b"cd", b" ", b"proy", b"\r", b"\r", b"pwd\r"],
     memuat("/proyek")),

    # Pemicu otomatis tidak boleh menyisipkan sendiri, meski kandidatnya
    # tinggal satu: ia mengubah baris perintah tanpa diminta.
    ("pemicu otomatis tidak menyisipkan sendiri",
     [b"cd", b" ", b"proy"],
     gabung(memuat("proyek/"), tanpa("cd proyek/"))),

    # Tab memang berarti "lengkapi lagi", jadi di sana menelusuri wajar.
    ("Tab pada direktori membuka isinya",
     [b"cd", b" ", b"proy", b"\t"],
     memuat("dalam")),

    # ------------------------------------------------------------------
    # Perilaku dasar yang harus selalu benar
    # ------------------------------------------------------------------
    ("baris tetap utuh setelah Esc", [b"git", b" ", b"comm", b"\x1b"], memuat("git comm")),
    ("backspace menyaring ulang", [b"git", b" ", b"commi", b"\x7f", b"\x7f"], memuat("commit")),
    # Menghapus sampai kosong TIDAK memunculkan kembali kotaknya: sesi sudah
    # menutup saat kandidat habis, dan pemicu berikutnya harus diketik. Yang
    # dijaga di sini adalah ketikannya tidak hilang.
    # Sesudah salah ketiklah saran paling dibutuhkan: "git commitx" tidak
    # cocok dengan apa pun dan kotaknya menutup; menghapus satu huruf harus
    # menampilkannya lagi.
    ("hapus huruf memunculkan kotak lagi",
     [b"git", b" ", b"commitx", b"\x7f"],
     memuat("Record changes to the repository")),
    # Satu penekanan backspace harus satu penghapusan. Pembungkusnya sempat
    # memakai status kembalian widget sebagai syarat, dan backward-delete-char
    # mengembalikan status bukan-nol saat kursor di awal baris — di situ
    # builtin-nya ikut dijalankan sesudahnya.
    #
    # Diperiksa lewat HASIL hitungannya: "50" tidak pernah diketik, jadi ia
    # hanya bisa muncul bila barisnya terpotong tepat tiga huruf.
    ("backspace menghapus tepat satu huruf",
     [b"echo $((20+30))zzz", b"\x7f", b"\x7f", b"\x7f", b"\r"],
     memuat("50")),
    ("backspace mempertahankan ketikan",
     [b"git", b" ", b"zz", b"\x7f"], memuat("git z")),
    ("mengetik yang tidak cocok menutup kotak",
     [b"git", b" ", b"zzqq"], gabung(memuat("git zzqq"), tanpa("commit"))),
    ("dua pemicu dalam satu baris",
     [b"git", b" ", b"commit", b" ", b"--am"], memuat("--amend")),
    # Sesudah pipa, posisinya adalah NAMA PERINTAH — dan nama perintah kini
    # dilengkapi dari PATH.
    ("pipa melengkapi nama perintah",
     [b"echo hai", b" ", b"|", b" ", b"gre"], memuat("grep")),
    # Keutuhan barisnya diperiksa dengan MENJALANKANNYA: "gre" yang dibiarkan
    # apa adanya harus sampai ke shell sebagai satu perintah yang dicari, bukan
    # terpotong atau tertukar oleh kotak yang sempat terbuka.
    ("pemicu sesudah pipa tidak merusak baris",
     [b"echo hai", b" ", b"|", b" ", b"gre", b"\x1b", b"\r"],
     memuat("command not found: gre")),
    ("opsi panjang dengan sama dengan",
     [b"kubectl", b" ", b"get", b" ", b"pods", b" ", b"--output", b"="], memuat("json")),

    # ------------------------------------------------------------------
    # cd dan jalur direktori
    # ------------------------------------------------------------------
    ("cd tidak menyisipkan saat dipicu", [b"cd", b" "],
     gabung(memuat("berkas/"), tanpa("cd berkas/"), tanpa("cd proyek/"))),
    ("cd menyaring lalu Enter menutup",
     [b"cd", b" ", b"proy", b"\r"], memuat("cd proyek/")),
    ("garis miring memicu isi direktori",
     [b"ls", b" ", b"proyek", b"/"], memuat("dalam")),
    ("path dua tingkat", [b"ls", b" ", b"proyek/dalam", b"/"], memuat("╭")),
    ("path absolut", [b"cat", b" ", b"/etc/hos"], memuat("hosts")),

    # ------------------------------------------------------------------
    # Tombol yang bukan urusan dropdown harus tetap berfungsi
    # ------------------------------------------------------------------
    ("Ctrl-C membatalkan baris", [b"git", b" ", b"comm", b"\x03", b"echo lanjut\r"],
     memuat("lanjut")),
    ("panah kiri menutup dan menggerakkan kursor",
     [b"git", b" ", b"comm", b"\x1b[D", b"X"], memuat("git comXm")),
    ("Ctrl-U membersihkan baris", [b"git", b" ", b"comm", b"\x15", b"echo bersih\r"],
     memuat("bersih")),

    # ------------------------------------------------------------------
    # Menjalankan perintah sungguhan lewat dropdown
    # ------------------------------------------------------------------
    ("perintah terpilih benar-benar jalan",
     [b"echo", b" ", b"catat", b"\r", b"\r"], memuat("catatan.txt")),
    ("Enter langsung tanpa memilih menjalankan apa adanya",
     [b"echo halo", b"\r"], memuat("halo")),

    # ------------------------------------------------------------------
    # Ketikan berat
    # ------------------------------------------------------------------
    # Diperiksa lewat KELUARAN perintahnya, bukan tampilan: yang harus benar
    # adalah isi buffer, dan tampilan bisa keliru karena hal lain.
    ("baris panjang sekaligus benar-benar utuh",
     [b"echo satu dua tiga empat lima enam tujuh delapan", b"\x1b", b"\r"],
     memuat("satu dua tiga empat lima enam tujuh delapan")),
    # Tempelan panjang pernah mengunci shell: setiap spasi di dalamnya membuka
    # sesi yang menunggu tombol yang sudah berada di penyangga zsh.
    ("shell tetap hidup setelah tempelan panjang",
     [b"echo satu dua tiga empat lima enam tujuh delapan", b"\x1b", b"\r",
      b"echo TANDA\r"],
     memuat("TANDA")),
    ("banyak pemicu beruntun tetap utuh",
     [b"echo", b" ", b"a", b" ", b"b", b" ", b"c", b" ", b"d", b"\x1b", b"\r"],
     memuat("a b c d")),

    ("bayangan hilang saat dropdown terbuka",
     [b"git", b" ", b"che"],
     tanpa("fitur-alpha")),
]


def main():
    saring = os.environ.get("SKENARIO", "")
    if not os.path.exists(os.path.join(REPO, "bin", "anjuran")):
        print("bin/anjuran belum ada; jalankan `make build` lebih dulu", file=sys.stderr)
        return 2

    bindir = siapkan_pemasangan()
    sandbox = siapkan_sandbox()
    print(f"pemasangan: {bindir}\nsandbox   : {sandbox}\n")

    h = Hasil()
    for nama, ketikan, periksa in SKENARIO:
        if saring and saring not in nama:
            continue
        # auto punya TIGA keadaan, dan nama skenario yang memilihnya:
        #   "tanpa disetel"      -> tidak menyetel apa pun; menguji bawaannya
        #   "tanpa mode otomatis"-> ANJURAN_AUTO=0; menguji Tab tetap bekerja
        #   selainnya            -> ANJURAN_AUTO=1
        # Skenario bayangan dimatikan otomatisnya supaya yang diuji benar-benar
        # mekanismenya, bukan interaksi keduanya.
        if "tanpa disetel" in nama:
            auto = None
        elif "tanpa mode otomatis" in nama or "bayangan" in nama:
            auto = False
        else:
            auto = True
        cachedir = tempfile.mkdtemp(prefix="anjuran-cache-")
        ghost = "bayangan" in nama
        persiapan = PERSIAPAN.get(nama, ())
        # Terminal pendek adalah kasusnya sendiri: di sana kotaknya bisa
        # menuntut ruang lebih banyak daripada yang tersedia.
        rows = 10 if "terminal pendek" in nama else 24
        alasan = jalankan(nama, ketikan, periksa, sandbox, bindir, cachedir,
                          auto=auto, ghost=ghost, persiapan=persiapan, rows=rows)
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
