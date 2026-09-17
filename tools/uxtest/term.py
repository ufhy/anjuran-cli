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
                        for y in range(self.row, self.rows):
                            for x in range(self.cols):
                                self.grid[y][x] = " "
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
                if self.col < self.cols:
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


def jawab_kueri(chunk):
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
        out.append("\x1b[1;1R")
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
            balas = jawab_kueri(teks)
            if balas:
                os.write(self.fd, balas)

    def ketik(self, data, jeda=0.7, diam=0.35):
        """Kirim ketikan, lalu tunggu sampai layarnya berhenti berubah.

        diam dipilih longgar dengan sengaja: membuka kotak berarti menjalankan
        proses anjuran yang baru — memindai PATH, memuat spec, menggambar — dan
        jeda di tengah rangkaian itu tidak boleh disalahartikan sebagai
        "sudah selesai".
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
        os.write(self.fd, ('echo SIA""P%d\r' % os.getpid()).encode())
        self.tunggu(batas, sampai=tanda)
        self.bersihkan_layar()

    def bersihkan_layar(self):
        self.layar = Layar(self.rows, self.cols)

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
