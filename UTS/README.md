# SIAKAD Mini API

RESTful API back end untuk layanan akademik sederhana: data mahasiswa, mata kuliah, dan Kartu Rencana Studi (KRS).
Dibuat untuk UTS Praktikum Pemrograman Backend Lanjut.

## Teknologi

- Go + [Fiber v2](https://docs.gofiber.io)
- PostgreSQL (driver [pgx v5](https://github.com/jackc/pgx))
- JWT (`golang-jwt/jwt/v5`) untuk autentikasi
- bcrypt untuk hashing password

## Struktur Folder

```
siakad-mini/
├── app/
│   ├── model/        struct entitas, request, dan response
│   ├── repository/   query ke database (SQL hanya ada di sini)
│   └── service/      penerima request (fiber.Ctx) + business rules
├── cmd/seed/         seeder (1 admin, 20 mahasiswa, 10 mata kuliah)
├── config/           env, instance Fiber, error handler
├── database/         koneksi PostgreSQL (connection pool)
├── helper/           response JSON, validasi, JWT, hashing
├── middleware/       RequireAuth, RequireRole, rate limiter login
├── migrations/       file SQL pembuatan tabel
├── route/            pendaftaran seluruh endpoint
├── requests.http     kumpulan request untuk pengujian (REST Client)
└── main.go
```

## Cara Menjalankan

1. Buat database:
```bash
   psql -d postgres -c "CREATE DATABASE siakad_mini;"
```
2. Jalankan migration secara berurutan:
```bash
   psql -d siakad_mini -f migrations/001_create_users.sql
   psql -d siakad_mini -f migrations/002_create_students.sql
   psql -d siakad_mini -f migrations/003_create_courses.sql
   psql -d siakad_mini -f migrations/004_create_enrollments.sql
```
3. Salin `.env.example` menjadi `.env`, lalu isi `DB_USER`, `DB_PASSWORD`, dan `JWT_SECRET`
   (buat secret dengan `openssl rand -hex 32`).
4. Install dependency dan jalankan seeder:
```bash
   go mod tidy
   go run ./cmd/seed
```
5. Jalankan server:
```bash
   go run .
```
   Server berjalan di `http://localhost:3000`.

## Akun Hasil Seeder

| Role      | Email                                | Password       |
|-----------|--------------------------------------|----------------|
| admin     | `admin@siakad.ac.id`                 | `admin12345`   |
| mahasiswa | `187221000001@student.siakad.ac.id`  | `187221000001` |

Seluruh 20 mahasiswa memakai pola yang sama: email `<nim>@student.siakad.ac.id`, password awal = NIM.

## Daftar Endpoint

Semua endpoint kecuali login wajib menyertakan header `Authorization: Bearer <token>`.

| No | Method | Endpoint                    | Akses                         | Status sukses |
|----|--------|-----------------------------|-------------------------------|---------------|
| 1  | POST   | `/api/v1/auth/login`        | Publik                        | 200 |
| 2  | GET    | `/api/v1/auth/me`           | Semua role                    | 200 |
| 3  | GET    | `/api/v1/students`          | Admin                         | 200 |
| 4  | POST   | `/api/v1/students`          | Admin                         | 201 |
| 5  | GET    | `/api/v1/students/{id}`     | Admin, mahasiswa (data sendiri) | 200 |
| 6  | PUT    | `/api/v1/students/{id}`     | Admin                         | 200 |
| 7  | DELETE | `/api/v1/students/{id}`     | Admin                         | 204 |
| 8  | GET    | `/api/v1/courses`           | Semua role                    | 200 |
| 9  | POST   | `/api/v1/enrollments`       | Mahasiswa                     | 201 |
| 10 | DELETE | `/api/v1/enrollments/{id}`  | Mahasiswa (milik sendiri)     | 204 |

Query parameter:
- `GET /students`: `page`, `per_page` (maks. 50), `prodi`, `angkatan`, `search` (nim/nama), `sort` (`nama`, `-nama`, `ipk_terakhir`, `-ipk_terakhir`)
- `GET /courses`: `semester`, `search` (kode_mk/nama_mk), `available=true`

## Business Rules

1. Batas SKS per semester: IPK ≥ 3,00 → 24 SKS; IPK 2,50–2,99 → 21 SKS; IPK < 2,50 → 18 SKS.
2. Mata kuliah yang sama tidak bisa diambil dua kali pada tahun akademik yang sama (409).
3. Mata kuliah yang kuotanya penuh tidak bisa diambil (422).
4. Mahasiswa hanya bisa melihat dan mengubah KRS miliknya sendiri (403).

Pengambilan mata kuliah dijalankan dalam satu transaction dengan row locking
(`SELECT ... FOR UPDATE`) pada baris mahasiswa dan mata kuliah, sehingga kuota tidak terlampaui
meskipun banyak mahasiswa mengambil mata kuliah yang sama secara bersamaan.

## Keamanan

- Password di-hash dengan bcrypt (cost 12).
- Login dibatasi 5 percobaan gagal per menit per IP (429).
- Pesan login gagal tidak membedakan "email tidak terdaftar" dan "password salah".
- Semua query memakai placeholder (`$1`, `$2`, ...), dan kolom sort memakai whitelist (aman dari SQL injection).
- Error 500 hanya dicatat di log server, tidak ada stack trace yang dikirim ke klien.
- Mahasiswa yang di-soft delete tidak muncul di daftar dan tidak bisa login.

## Pengujian

Buka `requests.http` di VS Code dengan extension **REST Client**. Login lewat request nomor 1 dan 3,
tempel token ke `@tokenMhs` dan `@tokenAdmin`, lalu klik **Send Request** di setiap request.