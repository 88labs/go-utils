package sql_escape

import "strings"

// EscapeLike escapes the special characters in the SQL Like statement.
// The escape character is regarded as '\\'.
func EscapeLike(s string) string {
	return EscapeLikeWithChar(s, '\\')
}

// EscapeLikeWithChar escapes the special characters in the SQL Like statement.
func EscapeLikeWithChar(s string, c rune) string {
	var b strings.Builder
	b.Grow(2 * (len(s)))

	start := 0
	for i, r := range s {
		switch r {
		case c, '%', '_':
			b.WriteString(s[start:i])
			b.WriteRune(c)
			b.WriteRune(r)
			start = i + 1
		}
	}
	b.WriteString(s[start:])

	return b.String()
}
