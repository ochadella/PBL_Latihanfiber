package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// LoadEnv memuat variabel dari berkas .env ke environment proses.
func LoadEnv() {
	if err := godotenv.Load(); err != nil {
		log.Println("peringatan: berkas .env tidak ditemukan, memakai environment sistem")
	}
}

// GetEnv membaca variabel environment, atau mengembalikan fallback bila kosong.
func GetEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// GetEnvInt sama seperti GetEnv, tetapi hasilnya berupa angka.
func GetEnvInt(key string, fallback int) int {
	v := GetEnv(key, "")
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
