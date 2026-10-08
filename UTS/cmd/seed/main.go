package main

import (
	"context"
	"fmt"
	"log"

	"siakad-mini/config"
	"siakad-mini/database"
	"siakad-mini/helper"
)

type seedStudent struct {
	NIM      string
	Nama     string
	Prodi    string
	Angkatan int
	IPK      float64
}

type seedCourse struct {
	Kode     string
	Nama     string
	SKS      int
	Semester int
	Kuota    int
}

func main() {
	config.LoadEnv()
	ctx := context.Background()

	pool, err := database.NewPool(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback(ctx) // diabaikan bila sudah Commit

	// Kosongkan semua tabel supaya seeder bisa dijalankan berulang kali.
	if _, err := tx.Exec(ctx,
		`TRUNCATE enrollments, students, courses, users RESTART IDENTITY CASCADE`); err != nil {
		log.Fatal("gagal mengosongkan tabel: ", err)
	}

	// ---------- 1 ADMIN ----------
	adminHash, err := helper.HashPassword("admin12345")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO users (email, password, role) VALUES ($1, $2, 'admin')`,
		"admin@siakad.ac.id", adminHash); err != nil {
		log.Fatal("gagal insert admin: ", err)
	}

	// ---------- 20 MAHASISWA ----------
	// Password awal setiap mahasiswa = NIM-nya sendiri (di-hash).
	// IPK sengaja dibuat bervariasi supaya ketiga batas SKS (24/21/18) bisa dites.
	students := []seedStudent{
		{"187221000001", "Rina Putri", "Sistem Informasi", 2022, 3.45},
		{"187221000002", "Budi Santoso", "Sistem Informasi", 2022, 2.75},
		{"187221000003", "Siti Aminah", "Sistem Informasi", 2022, 3.80},
		{"187221000004", "Andi Pratama", "Sistem Informasi", 2022, 2.30},
		{"187221000005", "Dewi Lestari", "Sistem Informasi", 2023, 3.10},
		{"187221000006", "Fajar Nugroho", "Sistem Informasi", 2023, 2.55},
		{"187221000007", "Galih Ramadhan", "Sistem Informasi", 2023, 3.95},
		{"187221000008", "Hana Safitri", "Teknik Informatika", 2022, 2.95},
		{"187221000009", "Irfan Maulana", "Teknik Informatika", 2022, 3.25},
		{"187221000010", "Jihan Aulia", "Teknik Informatika", 2023, 2.10},
		{"187221000011", "Kevin Wijaya", "Teknik Informatika", 2023, 3.60},
		{"187221000012", "Laras Setyawati", "Teknik Informatika", 2024, 2.80},
		{"187221000013", "Muhammad Rizki", "Teknik Informatika", 2024, 3.00},
		{"187221000014", "Nadia Kusuma", "Teknik Informatika", 2024, 2.45},
		{"187221000015", "Oka Saputra", "Sains Data", 2023, 3.70},
		{"187221000016", "Putri Anjani", "Sains Data", 2023, 2.60},
		{"187221000017", "Raka Firmansyah", "Sains Data", 2024, 3.15},
		{"187221000018", "Salsa Nabila", "Sains Data", 2024, 2.99},
		{"187221000019", "Taufik Hidayat", "Sains Data", 2025, 1.90},
		{"187221000020", "Umi Kalsum", "Sains Data", 2025, 3.50},
	}

	for _, s := range students {
		hash, err := helper.HashPassword(s.NIM)
		if err != nil {
			log.Fatal(err)
		}

		var userID int
		email := s.NIM + "@student.siakad.ac.id"
		if err := tx.QueryRow(ctx,
			`INSERT INTO users (email, password, role) VALUES ($1, $2, 'mahasiswa') RETURNING id`,
			email, hash).Scan(&userID); err != nil {
			log.Fatal("gagal insert user mahasiswa: ", err)
		}

		if _, err := tx.Exec(ctx,
			`INSERT INTO students (user_id, nim, nama, prodi, angkatan, ipk_terakhir)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			userID, s.NIM, s.Nama, s.Prodi, s.Angkatan, s.IPK); err != nil {
			log.Fatal("gagal insert student: ", err)
		}
	}

	// ---------- 10 MATA KULIAH ----------
	// MK010 sengaja kuotanya kecil (2) supaya mudah menguji "kuota penuh".
	courses := []seedCourse{
		{"MK001", "Pemrograman Backend Lanjut", 4, 5, 40},
		{"MK002", "Basis Data Lanjut", 3, 5, 40},
		{"MK003", "Rekayasa Perangkat Lunak", 3, 5, 35},
		{"MK004", "Jaringan Komputer", 3, 3, 35},
		{"MK005", "Kecerdasan Buatan", 3, 5, 30},
		{"MK006", "Interaksi Manusia dan Komputer", 2, 3, 30},
		{"MK007", "Keamanan Informasi", 3, 5, 30},
		{"MK008", "Statistika", 2, 3, 40},
		{"MK009", "Manajemen Proyek TI", 2, 7, 25},
		{"MK010", "Komputasi Awan", 3, 7, 2},
	}

	for _, c := range courses {
		if _, err := tx.Exec(ctx,
			`INSERT INTO courses (kode_mk, nama_mk, sks, semester, kuota) VALUES ($1, $2, $3, $4, $5)`,
			c.Kode, c.Nama, c.SKS, c.Semester, c.Kuota); err != nil {
			log.Fatal("gagal insert course: ", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatal("gagal commit: ", err)
	}

	fmt.Println("Seeder selesai:")
	fmt.Println("  - 1 admin      : admin@siakad.ac.id / admin12345")
	fmt.Printf("  - %d mahasiswa : <nim>@student.siakad.ac.id / password = nim\n", len(students))
	fmt.Printf("  - %d mata kuliah\n", len(courses))
}
