package kernel

import (
	"crypto/sha512"
	"fmt"
)

func Sha512(data string) string {
	return fmt.Sprintf("%032x", sha512.Sum512([]byte(data)))
}
