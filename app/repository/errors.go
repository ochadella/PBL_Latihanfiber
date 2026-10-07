package repository

import "errors"

// ErrNotFound dipakai semua repository bila data tidak ditemukan.
var ErrNotFound = errors.New("data tidak ditemukan")
