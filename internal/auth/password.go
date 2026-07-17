package auth

import "golang.org/x/crypto/bcrypt"

// bcryptCost is deliberately above the library default (10). 12 costs
// roughly 250ms per hash on typical API hardware — slow enough to blunt
// offline brute force, fast enough not to bottleneck login traffic.
const bcryptCost = 12

func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
