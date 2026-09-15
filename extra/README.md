# extra

Tambalan spec buatan tangan, **masuk git** dan tidak pernah dihasilkan mesin.

Isinya menambal argumen yang kosong pada spec bawaan. Fig memasok isi argumen
dinamis lewat closure JavaScript, dan 1.635 di antaranya tidak bisa dibawa saat
transpile — itulah sebabnya `kubectl logs <TAB>` dan `ssh <TAB>` tidak menawarkan
apa pun meski opsinya lengkap.

Berkas di sini DIGABUNG di atas spec bawaan, bukan menggantikannya: opsi,
subcommand, dan deskripsi dari Fig tetap utuh. Karena itu setiap berkas cukup
memuat bagian yang ditambal saja.

Generator di sini tunduk pada kebijakan yang sama seperti spec mana pun: hanya
perintah yang sedang diketik yang boleh dijalankan, dan tidak pernah lewat shell.
