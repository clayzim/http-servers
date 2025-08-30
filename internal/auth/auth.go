package auth

import (
	"errors"

	"github.com/alexedwards/argon2id"
)

// Constant parameters configured according to the
// second Argon2id settings from OWASP Cheat Sheet
// https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html#argon2id
// as of August 30, 2025

// Memory used (in kibibytes)
const memory uint32 = 19 * 1024

// Number of iterations
const iterations uint32 = 2

// The number of threads (or lanes) used by the algorithm.
// Recommended value is between 1 and runtime.NumCPU().
const parallelism uint8 = 1

// Length of the random salt in bytes
const saltLength uint32 = 16

// Length of the generated hash in bytes
const hashLength uint32 = 32

func HashPassword(password string) (string, error) {
	params := argon2id.Params {
		Memory: memory,
		Iterations: iterations,
		Parallelism: parallelism,
		SaltLength: saltLength,
		KeyLength: hashLength,
	}
	return argon2id.CreateHash(password, &params)
}

var ErrMismatchedHashAndPassword = errors.New("internal/auth: Given hash is not the hash of the given password")

func CheckPassword(password, hash string) error {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if match {
		return err
	} else {
		return ErrMismatchedHashAndPassword
	}
}