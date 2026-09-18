#!/usr/bin/env python3
"""Menggambar assets/social-preview.png, gambar pratinjau sosial GitHub.

Digambar dengan kode, bukan disalin dari tangkapan layar, supaya bisa dibuat
ulang saat tampilannya berubah — dan supaya ukurannya persis 1280x640 seperti
yang diminta GitHub, tanpa bergantung pada konverter SVG yang kebetulan ada.
"""
from PIL import Image, ImageDraw, ImageFont
import os

W, H = 1280, 640
LATAR = (11, 15, 20)
HIJAU = (22, 163, 74)
TERANG = (243, 244, 246)
ABU = (156, 163, 175)
ABU_TUA = (107, 114, 128)
GELAP = (5, 46, 22)
GARIS = (55, 65, 81)

def font(nama, ukuran):
    for p in (f"/System/Library/Fonts/{nama}", f"/Library/Fonts/{nama}"):
        if os.path.exists(p):
            return ImageFont.truetype(p, ukuran)
    return ImageFont.load_default()

mono = lambda n: font("Menlo.ttc", n)
tebal = lambda n: font("SFNSDisplay.ttf", n) if os.path.exists("/System/Library/Fonts/SFNSDisplay.ttf") else mono(n)

img = Image.new("RGB", (W, H), LATAR)
d = ImageDraw.Draw(img)

d.text((88, 86), "anjuran", font=mono(64), fill=TERANG)
d.text((92, 176), "Shell autocomplete that works over SSH", font=mono(26), fill=ABU)

d.text((88, 268), "$", font=mono(22), fill=(74, 222, 128))
d.text((116, 268), "git", font=mono(22), fill=ABU)

# Kotak completion: bentuk yang memang digambar alat ini.
d.rounded_rectangle((88, 312, 1192, 528), radius=12, outline=GARIS, width=2)
d.rounded_rectangle((98, 322, 1182, 360), radius=7, fill=HIJAU)

baris = [
    ("❯ ▪ commit",      "Record changes to the repository",   GELAP, 331),
    ("  ▪ checkout",         "Switch branches or restore files",   ABU,   381),
    ("  ▪ cherry-pick",      "Apply changes from existing commits", ABU,  423),
    ("  ▪ clone",            "Clone a repository into a directory", ABU,  465),
]
for kiri, kanan, warna, y in baris:
    d.text((118, y), kiri, font=mono(22), fill=warna)
    d.text((430, y), kanan, font=mono(22), fill=GELAP if warna is GELAP else ABU_TUA)

d.text((88, 566), "zsh · bash · fish · PowerShell", font=mono(20), fill=ABU_TUA)
d.text((560, 566), "Linux · macOS · Windows", font=mono(20), fill=ABU_TUA)

keluar = os.path.join(os.path.dirname(__file__), "..", "..", "assets", "social-preview.png")
img.save(os.path.normpath(keluar))
print("ditulis:", os.path.normpath(keluar), img.size)
