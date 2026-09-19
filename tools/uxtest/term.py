"""Terminal palsu untuk menguji anjuran di dalam shell sungguhan.

Menjalankan zsh asli di dalam PTY, mengirim ketikan, lalu MEMUTAR ULANG escape
sequence yang keluar menjadi kisi teks — sehingga yang diperiksa adalah apa
yang benar-benar terlihat pengguna, bukan keluaran API.

Ini dibuat setelah berkali-kali menyimpulkan "tidak jalan" padahal harness
sekali pakai yang salah: newline yang tidak dimodelkan, penggulungan layar yang
diabaikan, HOME yang diarahkan ke tempat lain sehingga konfigurasi asli tidak
termuat. Setiap kekeliruan itu dijawab di berkas ini, sekali.
"""

import os
import pty
import re
import select
import signal
import time


class Layar:
    """Kisi teks hasil pemutaran ulang escape sequence."""

    def __init__(self, rows, cols):
        self.rows, self.cols = rows, cols
        self.grid = [[" "] * cols for _ in range(rows)]
        self.row = self.col = 0
        self.saved = [0, 0]

    def _gulung(self):
        # Baris teratas hilang; posisi tersimpan ikut naik bersama isinya.
        self.grid.pop(0)
        self.grid.append([" "] * self.cols)
        self.saved[0] = max(0, self.saved[0] - 1)

    def _turun(self):
        if self.row >= self.rows - 1:
            self._gulung()
        else:
            self.row += 1

    def tulis(self, teks):
        i = 0
        while i < len(teks):
            c = teks[i]

            if c == "\x1b":
                # ESC D menurunkan kursor TANPA mengubah kolomnya.
                if teks[i:i + 2] == "\x1bD":
                    self._turun()
                    i += 2
                    continue
                if teks[i:i + 2] == "\x1b7":
                    self.saved = [self.row, self.col]
                    i += 2
                    continue
                if teks[i:i + 2] == "\x1b8":
                    self.row, self.col = self.saved[0], self.saved[1]
                    i += 2
                    continue
                # Pemilih charset: ESC ( B dan kerabatnya. Tiga byte, tanpa
                # pengaruh pada gambar. Tanpa dilompati, dua byte sesudah ESC
                # tercetak sebagai teks dan setiap baris fish terbaca sebagai
                # ketikan yang berantakan — "(B" bertaburan di layar.
                if teks[i:i + 1] == "\x1b" and teks[i + 1:i + 2] in "()*+":
                    i += 3
                    continue
                # OSC diakhiri BEL atau ST; isinya bukan gambar.
                if teks[i:i + 2] == "\x1b]":
                    j = teks.find("\x07", i)
                    i = j + 1 if j > 0 else i + 2
                    continue

                m = re.match(r"\x1b\[([0-9;?]*)([A-Za-z])", teks[i:])
                if m:
                    arg, op = m.group(1), m.group(2)
                    n = int(arg) if arg.isdigit() and arg else 1
                    if op == "A":
                        self.row = max(0, self.row - n)
                    elif op == "B":
                        for _ in range(n):
                            self._turun()
                    elif op == "C":
                        self.col = min(self.cols - 1, self.col + n)
                    elif op == "D":
                        self.col = max(0, self.col - n)
                    elif op == "K":
                        for x in range(self.col, self.cols):
                            self.grid[self.row][x] = " "
                    elif op == "J":
                        # 0 (bawaan) sampai akhir layar, 1 sampai awal layar,
                        # 2 dan 3 seluruhnya. Sebelumnya n diabaikan dan
                        # SEMUA bentuk diperlakukan sebagai 0, sehingga
                        # ESC[2J — yang dikirim `clear` dan sebagian prompt —
                        # meninggalkan seluruh isi di atas kursor.
                        mode = int(arg) if arg.isdigit() else 0
                        mulai = 0 if mode in (1, 2, 3) else self.row
                        akhir = self.row + 1 if mode == 1 else self.rows
                        for y in range(mulai, akhir):
                            for x in range(self.cols):
                                self.grid[y][x] = " "
                    elif op == "G":
                        # Kolom ABSOLUT, dihitung dari 1. fish memakainya di
                        # setiap penggambaran ulang; tanpa dimodelkan, teksnya
                        # mendarat di kolom yang salah dan seluruh layar
                        # terbaca seperti ketikan yang berantakan.
                        self.col = max(0, min(self.cols - 1, n - 1))
                    elif op in ("H", "f"):
                        # Baris dan kolom absolut, keduanya dihitung dari 1.
                        bagian = (arg or "").split(";")
                        r = int(bagian[0]) if bagian[0].isdigit() else 1
                        c = int(bagian[1]) if len(bagian) > 1 and bagian[1].isdigit() else 1
                        self.row = max(0, min(self.rows - 1, r - 1))
                        self.col = max(0, min(self.cols - 1, c - 1))
                    i += m.end()
                    continue
                i += 1
                continue

            if c == "\r":
                self.col = 0
            elif c == "\n":
                self._turun()
            elif c == "\b":
                self.col = max(0, self.col - 1)
            elif c == "\x07":
                # BEL berbunyi, tidak menggambar. Memperlakukannya sebagai
                # karakter biasa menyisipkan \x07 ke tengah baris, dan
                # kegagalan asersi yang diakibatkannya tampak seperti teka-teki
                # — layarnya terlihat benar padahal tidak cocok.
                pass
            elif c in ("\x00", "\x0e", "\x0f"):
                # NUL dan pemilih charset juga tidak menggambar apa pun.
                pass
            else:
                # Baris yang penuh BERGULUNG ke baris berikutnya.
                #
                # Sebelumnya kolom di luar lebar layar hanya dibuang dan
                # kursornya tidak pernah turun. Setiap baris yang lebih
                # panjang dari lebar terminal — dan perintah persiapan di
                # rangkaian ini memang begitu — membuat model ini tertinggal
                # satu baris atau lebih dari shell-nya. Sesudah itu setiap
                # penempatan mutlak meleset sejauh selisih itu.
                if self.col >= self.cols:
                    self.col = 0
                    self._turun()
                self.grid[self.row][self.col] = c
                self.col += 1
            i += 1

    def baris(self):
        out = ["".join(r).rstrip() for r in self.grid]
        while out and not out[-1]:
            out.pop()
        return out

    def teks(self):
        return "\n".join(self.baris())


def potong_tak_lengkap(teks):
    """Pisahkan teks menjadi (utuh, sisa) pada escape sequence yang terpotong."""
    i = teks.rfind("\x1b")
    if i < 0:
        return teks, ""
    ekor = teks[i:]

    # OSC: ESC ] ... diakhiri BEL atau ST.
    if ekor.startswith("\x1b]"):
        if "\x07" in ekor or "\x1b\\" in ekor[2:]:
            return teks, ""
        return teks[:i], ekor
    # CSI: ESC [ parameter lalu satu huruf penutup.
    if ekor.startswith("\x1b["):
        if re.match(r"\x1b\[[0-9;?]*[ -/]*[@-~]", ekor):
            return teks, ""
        return teks[:i], ekor
    # ESC sendirian di ujung: penutupnya belum datang.
    if len(ekor) == 1:
        return teks[:i], ekor
    return teks, ""


# Kueri kemampuan terminal yang DITUNGGU jawabannya oleh shell modern.
# Tanpa menjawabnya, fish dan powershell menggantung sebelum prompt muncul.
_ST = "\x1b\\"


def jawab_kueri(chunk, posisi=None):
    """Susun jawaban atas kueri kemampuan yang ada di chunk.

    posisi adalah (baris, kolom) berbasis nol milik emulator, dipakai untuk
    menjawab CPR.
    """
    out = []
    if re.search(r"\x1b\[(0)?c", chunk):
        out.append("\x1b[?62;1;6c")
    if "\x1b[>0q" in chunk:
        out.append("\x1bP>|uxtest(0)" + _ST)
    if "\x1b[?u" in chunk:
        out.append("\x1b[?0u")
    for n in ("10", "11"):
        if f"\x1b]{n};?" in chunk:
            out.append(f"\x1b]{n};rgb:0000/0000/0000" + _ST)
    if "\x1b[6n" in chunk:
        # CPR dijawab dengan posisi kursor yang SEBENARNYA, bukan 1;1.
        #
        # Shell menanyakan ini untuk mengetahui di mana ia berada sebelum
        # menggambar ulang. Jawaban tetap 1;1 adalah kebohongan yang kebetulan
        # benar hanya di baris pertama; di baris mana pun selain itu ia
        # menyesatkan, dan emulator yang berbohong tidak bisa dipakai untuk
        # menilai gambar orang lain.
        baris, kolom = posisi or (0, 0)
        out.append("\x1b[%d;%dR" % (baris + 1, kolom + 1))
    # XTGETTCAP sengaja TIDAK dijawab: jawaban yang tidak dikenali shell
    # tersisa di buffer masukan dan ikut terketik sebagai perintah.
    return "".join(out).encode()


class Sesi:
    """Satu sesi shell di dalam PTY."""

    def __init__(self, argv, cwd, env=None, rows=24, cols=100):
        self.rows, self.cols = rows, cols
        self.pid, self.fd = pty.fork()
        if self.pid == 0:
            os.chdir(cwd)
            os.environ["TERM"] = "xterm-256color"
            if env:
                os.environ.update(env)
            os.execv(argv[0], argv)

        self._ukuran(rows, cols)
        self.layar = Layar(rows, cols)

    def _ukuran(self, rows, cols):
        # Tanpa ini TIOCGWINSZ melaporkan 0x0, dan PSReadLine membagi dengan nol.
        import fcntl
        import struct
        import termios
        fcntl.ioctl(self.fd, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))

    # _sisa menampung escape sequence yang TERPOTONG di ujung chunk.
    #
    # Satu pembacaan tidak dijamin berisi urutan yang utuh, dan menyerahkan
    # potongan ke emulator membuat sisanya digambar sebagai teks biasa. Hook
    # prompt kiro-cli mengirim OSC 697 pada setiap prompt, sehingga tanpa ini
    # potongan seperti "=97;NewCmd=..." muncul di tengah baris dan setiap
    # pengukuran layar jadi tidak bisa dipercaya.
    _sisa = ""

    def tunggu(self, detik, diam=None, sampai=None):
        """Baca keluaran sampai salah satu berhenti terjadi.

        Tiga cara berhenti, dari yang paling pasti ke yang paling kasar:

        - ``sampai``: teks itu muncul di layar. Dipakai bila ada penanda yang
          benar-benar menjawab "sudah siap?" — jauh lebih baik daripada
          menebak.
        - ``diam``: keluarannya berhenti selama sekian detik. Dipakai saat
          tidak ada penanda, misalnya sesudah satu ketikan.
        - ``detik``: batas atas, jaring pengaman.

        Sebelumnya hanya ada batas atas, dan ia SELALU ditunggu sampai habis
        walaupun shell-nya sudah siap sejak seperlima detik pertama. Dua
        pertiga waktu seluruh rangkaian uji habis di situ — bukan menguji,
        melainkan tidur. Menunggu keadaan alih-alih angka juga lebih andal:
        di mesin yang sedang sibuk ia otomatis menunggu lebih lama, sedangkan
        angka tetap justru gagal persis di situ.
        """
        akhir = time.time() + detik
        terakhir = time.time()
        while time.time() < akhir:
            if sampai is not None and sampai in self.layar.teks():
                return
            r, _, _ = select.select([self.fd], [], [], 0.02)
            if not r:
                if diam is not None and time.time() - terakhir >= diam:
                    return
                continue
            terakhir = time.time()
            try:
                chunk = os.read(self.fd, 65536)
            except OSError:
                return
            if not chunk:
                return
            teks = self._sisa + chunk.decode(errors="replace")
            teks, self._sisa = potong_tak_lengkap(teks)
            self.layar.tulis(teks)
            # Dijawab SESUDAH teksnya diputar ulang, supaya posisi yang
            # dilaporkan adalah posisi sesudah keluaran itu tergambar —
            # persis seperti yang dilihat terminal sungguhan.
            balas = jawab_kueri(teks, (self.layar.row, self.layar.col))
            if balas:
                os.write(self.fd, balas)

    def ketik(self, data, jeda=3.0, diam=0.35):
        """Kirim ketikan, lalu tunggu sampai layarnya berhenti berubah.

        diam dipilih longgar dengan sengaja: membuka kotak berarti menjalankan
        proses anjuran yang baru — memindai PATH, memuat spec, menggambar — dan
        jeda di tengah rangkaian itu tidak boleh disalahartikan sebagai
        "sudah selesai".

        jeda hanyalah batas atas, dan sengaja dibuat besar. Ia tidak pernah
        dibayar saat mesinnya lengang — yang menentukan adalah diam. Batas yang
        ketat justru berbahaya: saat mesin sibuk, ia memotong kotak yang sedang
        digambar dan menghasilkan kegagalan yang tidak ada hubungannya dengan
        kode.
        """
        os.write(self.fd, data if isinstance(data, bytes) else data.encode())
        self.tunggu(jeda, diam=diam)

    def siap(self, batas=10.0):
        """Tunggu sampai shell benar-benar siap menerima perintah.

        Dijawab dengan penanda, bukan dengan tebakan: sebuah echo yang
        keluarannya ditunggu. Itu satu-satunya cara yang benar-benar menjawab
        "sudah siap?" — prompt bisa digambar berkali-kali oleh p10k, dan
        instant prompt-nya muncul jauh sebelum konfigurasi selesai dimuat.
        """
        # Penandanya disusun agar HANYA cocok dengan keluarannya, tidak dengan
        # gema ketikannya: yang diketik memuat tanda kutip, yang dicetak tidak.
        # Tanpa pemisahan ini, penanda cocok begitu barisnya tergema — sebelum
        # shell benar-benar mengerjakan apa pun.
        tanda = "SIAP%d" % os.getpid()
        # Penandanya sendiri MEMICU anjuran: ia memuat spasi, dan spasi adalah
        # tombol pemicu. Kotak yang terbuka karenanya menangkap Enter sebagai
        # "pilih kandidat", bukan "jalankan baris" — sehingga perintah penanda
        # tidak pernah dijalankan dan menumpuk di baris masukan bersama
        # ketikan skenario berikutnya. Yang terbaca lalu berupa
        # '% echo SIA""P2486echo SIA""P2486cd', dan seluruh skenario sesudahnya
        # gagal karena alasan yang tidak ada hubungannya dengan yang diuji.
        #
        # Esc ditekan lebih dulu, persis seperti yang dilakukan orang.
        os.write(self.fd, ('echo SIA""P%d' % os.getpid()).encode())
        self.tunggu(2.0, diam=0.3)
        if "╭" in self.layar.teks():
            os.write(self.fd, b"\x1b")
            # Jeda PENUH, bukan sampai keluarannya diam.
            #
            # Esc yang menutup kotak tidak menghasilkan keluaran apa pun, jadi
            # menunggu "diam" selesai dalam sepersekian detik — lebih cepat
            # daripada KEYTIMEOUT zsh yang 0,4 detik. Esc dan Enter lalu tiba
            # dalam satu bacaan dan zsh membacanya sebagai satu urutan meta,
            # bukan dua tombol. Dua puluh empat skenario zsh gagal karena itu.
            self.tunggu(0.7)
        os.write(self.fd, b"\r")
        self.tunggu(batas, sampai=tanda)
        # Penanda yang muncul belum berarti shell-nya selesai menggambar.
        #
        # fish menggambar ulang baris masukannya secara TERTUNDA — penyorotan
        # sintaks dan saran otomatisnya dihitung sesudah barisnya dikirim —
        # sehingga cat ulang baris sebelumnya tiba beberapa puluh milidetik
        # setelah keluaran penandanya. Tanpa jeda ini cat ulang itu mendarat
        # sesudah layar dikosongkan, lalu bercampur dengan ketikan skenario:
        # yang terbaca menjadi 'ececho SIA""P2497echo $((20+30))'. Kegagalan
        # seperti itu tampak seperti cacat anjuran, padahal milik harness.
        self.tunggu(1.5, diam=0.3)
        self.bersihkan_layar()

    def bersihkan_layar(self):
        """Kosongkan riwayat layar TANPA memindahkan kursor atau baris aktif.

        Dulu ini mengganti seluruh Layar dengan yang baru, sehingga kursornya
        kembali ke 0;0 dan prompt yang sedang tampil ikut hilang. Itu memutus
        kesepakatan dengan shell-nya: fish memposisikan ulang kursor secara
        MUTLAK per kolom — ia mengirim '\\r' lalu 'ESC[5C' untuk kembali ke
        ujung "git" di belakang prompt dua karakter. Dengan prompt yang sudah
        dihapus, kolom 5 di model kita menunjuk dua karakter terlalu jauh, dan
        hasilnya terbaca sebagai 'gigit commit --am'.

        zsh dan bash tidak terpengaruh karena keduanya menggambar ulang
        promptnya sendiri di setiap perubahan, sehingga apa pun yang hilang
        segera dikembalikan. Itulah sebabnya cacat ini hanya tampak di fish,
        dan tampak seperti cacat anjuran.
        """
        for r in range(self.rows):
            if r != self.layar.row:
                self.layar.grid[r] = [" "] * self.cols

    def tutup(self):
        try:
            os.kill(self.pid, signal.SIGKILL)
            os.waitpid(self.pid, 0)
        except Exception:
            pass
        try:
            os.close(self.fd)
        except Exception:
            pass
