// Package phone normalizes, validates and formats Brazilian phone numbers
// used as the client login credential, e.g. (99) 99999-9999.
package phone

import (
	"errors"
	"strings"
)

var ErrInvalid = errors.New("telefone inválido")

func Normalize(s string) (string, error) {
	digits := strings.Builder{}
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	if len(d) != 10 && len(d) != 11 {
		return "", ErrInvalid
	}
	if d[0] == '0' {
		return "", ErrInvalid
	}
	return d, nil
}

func Format(digits string) string {
	if len(digits) == 11 {
		return "(" + digits[0:2] + ") " + digits[2:7] + "-" + digits[7:11]
	}
	if len(digits) == 10 {
		return "(" + digits[0:2] + ") " + digits[2:6] + "-" + digits[6:10]
	}
	return digits
}
