# extra

Tambalan spec buatan tangan, **masuk git** dan tidak pernah dihasilkan mesin.
Di sinilah sumbangan diterima.

Isinya menambal argumen yang kosong pada spec bawaan. Fig memasok isi argumen
dinamis lewat closure JavaScript, dan 1.635 di antaranya tidak bisa dibawa saat
transpile — itulah sebabnya `kubectl logs <TAB>` dan `ssh <TAB>` tidak menawarkan
apa pun meski opsinya lengkap.

Berkas di sini **digabung di atas** spec bawaan, bukan menggantikannya: opsi,
subcommand, dan deskripsi dari Fig tetap utuh. Karena itu setiap berkas cukup
memuat bagian yang ditambal saja.

Korpus hulu berhenti dirawat pada Mei 2025. Pembaruan yang masih mungkin datang
dari orang yang memakai perintahnya sehari-hari — yaitu dari sini.

## Kenapa direktori ini ditinjau satu per satu

Spec di `extra/` ditandai **trusted**, dan penandaan itu melonggarkan satu
larangan: generator di sini boleh memanggil interpreter, yang di korpus hasil
transpile selalu ditolak.

Kelonggaran itu ada karena larangannya sebetulnya menyasar *argumen yang isinya
kode*, dan nama biner hanyalah perkiraan kasar untuk itu — `php artisan list`
bukan kode, sedangkan `php -r <apa pun>` jelas kode. Yang membedakan keduanya
bukan binernya, melainkan siapa yang menulis argv-nya.

Konsekuensinya lugas: **menerima spec berarti menerima perintah yang akan
dijalankan di mesin orang lain, sebagai efek samping mengetik.** Bukan saat
seseorang menekan Enter — saat ia menekan spasi.

Karena itu setiap berkas di sini dibaca manusia sebelum masuk. Tetapi peninjau
bisa lelah dan bisa terburu-buru, jadi ada lapisan kedua yang tidak bisa.

## Yang diperiksa otomatis

`go test ./internal/engine/ -run TestExtra` menolak, untuk setiap berkas:

| Pemeriksaan | Kenapa |
|---|---|
| nama berkas sama dengan nama perintahnya | penggabungan memakai nama berkas; yang meleset tidak pernah terpakai, dan gagalnya diam total |
| generator hanya menjalankan perintahnya sendiri | aturan yang sama yang berlaku saat dijalankan, diperiksa saat spec masuk |
| tidak ada flag yang argumennya kode (`-c`, `-e`, `-r`, `--eval`, …) | bentuk paling berbahaya dari kelonggaran interpreter |
| tidak ada metakarakter shell (`\|`, `;`, `&`, `$`, `` ` ``) di argumen | argv dijalankan langsung, tanpa shell; kehadirannya menandakan anggapan yang keliru tentang eksekusi |
| menjawab setiap baris contohnya | spec yang terurai bersih tetapi diam adalah kegagalan yang paling mahal ditemukan |

Pemeriksaan ini berjalan di CI untuk setiap perubahan.

## Menulis sebuah spec

Satu berkas, dinamai persis seperti perintahnya:

```json
{
  "name": "kubectl",
  "anjuranContoh": ["kubectl logs "],
  "subcommands": [
    {
      "name": "logs",
      "args": [
        {
          "name": "pod",
          "generators": [
            { "script": ["kubectl", "get", "pods", "-o", "name"], "cacheTtl": 5 }
          ]
        }
      ]
    }
  ]
}
```

`anjuranContoh` wajib. Isinya baris perintah yang **harus** menjawab sesuatu,
ditulis pada posisi yang justru ditambal spec ini — bukan baris yang kebetulan
menjawab karena spec bawaannya sudah lengkap. Contohnya tinggal di berkas yang
sama dengan specnya supaya keduanya tidak pernah berpisah; contoh yang terpisah
akan lupa diperbarui saat specnya berubah.

Isi argumen bisa datang dari tiga tempat:

- **`generators`** — menjalankan perintahnya untuk menanyakan jawabannya.
  Beri `cacheTtl` dalam detik; menampilkan nama pod yang sudah mati selama
  setengah menit lebih buruk daripada menunggu sebentar.
- **`suggestions`** — daftar tetap, untuk nilai yang memang tidak berubah.
- **`template`** — `filepaths` atau `folders`, dikerjakan tanpa menjalankan
  proses apa pun.

`whenFile` membuat sebuah entri hanya muncul bila berkas itu ada di direktori
kerja. `php artisan` hanya bermakna di proyek Laravel; menawarkannya di mana-mana
membuat daftarnya berbohong tentang apa yang bisa dijalankan.

## Yang tidak akan diterima

- Generator yang memanggil program selain perintahnya sendiri
- Apa pun yang menjalankan kode dari argumen — `bash -c`, `python -c`, `-eval`
- Apa pun yang menyentuh jaringan: `curl`, `wget`, `nc`
- Spec tanpa `anjuranContoh`
- Generator berat tanpa `cacheTtl`; jalur ini dipanggil setiap tombol pemicu
  ditekan

## Sebelum mengirim

```sh
go test ./internal/engine/ -run TestExtra
make build && ./bin/anjuran complete --line "perintahmu "
```

Yang kedua penting: uji membuktikan specnya menjawab, sedangkan perintah itu
memperlihatkan **apa** yang dijawabnya.
