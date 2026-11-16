package onec

import (
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

func russianTextScore(s string) int {
	score := 0
	for _, r := range s {
		switch {
		case (r >= 'А' && r <= 'я'):
			score += 2
		case r == 'Ё' || r == 'ё':
			score += 1
		case r >= 0x2500 && r <= 0x259F: // box-drawing / block elements
			score -= 4
		case r == utf8.RuneError:
			score -= 5
		}
	}
	return score
}

// DecodeConsoleOutput converts raw bytes from Windows CLI tools (such as 1C rac.exe)
// into valid UTF-8, handling UTF-8 (with or without BOM), OEM CP866, and Windows-1251.
func DecodeConsoleOutput(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	// Strip UTF-8 BOM if present
	if len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF {
		raw = raw[3:]
	}
	if utf8.Valid(raw) {
		return string(raw)
	}

	s866, err866 := charmap.CodePage866.NewDecoder().Bytes(raw)
	s1251, err1251 := charmap.Windows1251.NewDecoder().Bytes(raw)

	if err866 == nil && err1251 == nil {
		str866 := string(s866)
		str1251 := string(s1251)
		if russianTextScore(str1251) > russianTextScore(str866) {
			return str1251
		}
		return str866
	}
	if err866 == nil {
		return string(s866)
	}
	if err1251 == nil {
		return string(s1251)
	}
	return string(raw)
}
