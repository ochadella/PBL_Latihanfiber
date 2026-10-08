package helper

import "golang.org/x/crypto/bcrypt"

// bcryptCost menentukan seberapa lambat proses hashing (makin besar makin aman, makin lambat).
const bcryptCost = 12

// HashPassword mengubah password biasa menjadi hash bcrypt.
func HashPassword(plain string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// CheckPassword mencocokkan password biasa dengan hash yang tersimpan.
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
