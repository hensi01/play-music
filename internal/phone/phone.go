// Package phone normalizes, validates and formats Brazilian phone numbers
// used as the client login credential, e.g. (99) 99999-9999.
package phone

import (
	"errors"
	"strings"
)

var ErrInvalid = errors.New("telefone inválido")

// Normalize strips formatting and validates: only digits, 10 or 11 digits
// (DDD + number). Returns the canonical digit-only form.
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

// Format renders a normalized (digit-only) phone as "(XX) XXXXX-XXXX" or
// "(XX) XXXX-XXXX" for 10-digit numbers.
func Format(digits string) string {
	if len(digits) == 11 {
		return "(" + digits[0:2] + ") " + digits[2:7] + "-" + digits[7:11]
	}
	if len(digits) == 10 {
		return "(" + digits[0:2] + ") " + digits[2:6] + "-" + digits[6:10]
	}
	return digits
}

// Mask accepts raw input and returns the formatted phone while typing.
// The raw value may be partial (0-11 digits); formatting is best-effort.
func Mask(raw string) string {
	d := strings.Builder{}
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			d.WriteRune(r)
		}
	}
	digits := d.String()
	out := strings.Builder{}
	for i := 0; i < len(digits); i++ {
		switch i {
		case 0:
			out.WriteByte('(')
		case 2:
			out.WriteByte(' ')
		case 6:
			if len(digits) == 10 {
				out.WriteByte('-')
			}
		case 7:
			if len(digits) > 10 {
				out.WriteByte('-')
			}
		}
		out.WriteByte(digits[i])
		if i == 1 {
			out.WriteByte(')')
		}
	}
	return out.String()
}
