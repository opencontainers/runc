// Package utils contains small internal helpers that do not belong to a
// specific runc subsystem.
package utils

import (
	"errors"
	"strings"
)

var (
	// ErrUnclosedDoubleQuote is returned when a value opened with a double
	// quote is never closed.
	ErrUnclosedDoubleQuote = errors.New("unclosed double quote")
	// ErrUnclosedSingleQuote is returned when a value opened with a single
	// quote is never closed.
	ErrUnclosedSingleQuote = errors.New("unclosed single quote")
	// ErrUnfinishedEscape is returned when the input ends with a backslash.
	ErrUnfinishedEscape = errors.New("unfinished escape sequence")
)

// ParseKV parses a comma-separated list of key=value pairs into a map.
//
// A key without '=' is stored with an empty value, and pairs with an empty
// key are skipped. Commas inside a value are kept as is when either the
// value is wrapped in matching quotes (a quote only acts as a wrapper if it
// is the first character after '=') or the comma is escaped with a
// backslash. A backslash takes the following character literally anywhere
// in the input, so `\,` and `\\` produce a literal comma and backslash
// respectively. Whitespace is significant and never trimmed.
func ParseKV(s string) (map[string]string, error) {
	result := make(map[string]string)
	if s == "" {
		return result, nil
	}

	var (
		key, value strings.Builder
		inKey      = true // parsing the key rather than the value
		quote      rune   // active quote character, 0 outside quotes
		escaped    bool   // next character is escaped
	)

	write := func(r rune) {
		if inKey {
			key.WriteRune(r)
		} else {
			value.WriteRune(r)
		}
	}

	flush := func() {
		if k := key.String(); k != "" {
			result[k] = value.String()
		}
		key.Reset()
		value.Reset()
		inKey = true
		quote = 0
	}

	for _, r := range s {
		switch {
		case escaped:
			// The escaped character is taken literally.
			write(r)
			escaped = false

		case r == '\\':
			escaped = true

		case r == '"' || r == '\'':
			switch {
			case quote == r:
				// Closing quote.
				quote = 0
			case quote != 0:
				// A quote of the other kind is a literal character.
				write(r)
			case !inKey && value.Len() == 0:
				// A quote wraps the value only right after '='.
				quote = r
			default:
				write(r)
			}

		case r == '=':
			// The first '=' outside quotes separates key and value.
			if inKey && quote == 0 {
				inKey = false
			} else {
				write(r)
			}

		case r == ',':
			if quote == 0 {
				flush()
			} else {
				write(r)
			}

		default:
			write(r)
		}
	}

	if quote == '"' {
		return nil, ErrUnclosedDoubleQuote
	}
	if quote == '\'' {
		return nil, ErrUnclosedSingleQuote
	}
	if escaped {
		return nil, ErrUnfinishedEscape
	}

	flush()

	return result, nil
}
