package onec

import (
	"testing"

	"golang.org/x/text/encoding/charmap"
)

func TestDecodeConsoleOutputRussianEncodings(t *testing.T) {
	sample := "user-name : Иванов Иван Иванович\r\ndescr : \"Бухгалтерия предприятия 3.0\"\r\n"

	// 1. UTF-8
	if got := DecodeConsoleOutput([]byte(sample)); got != sample {
		t.Fatalf("UTF-8 mismatch: got %q", got)
	}

	// 2. OEM CP866
	cp866Bytes, err := charmap.CodePage866.NewEncoder().Bytes([]byte(sample))
	if err != nil {
		t.Fatalf("encode CP866: %v", err)
	}
	if got := DecodeConsoleOutput(cp866Bytes); got != sample {
		t.Fatalf("CP866 decode mismatch: got %q, want %q", got, sample)
	}

	// 3. Windows-1251
	win1251Bytes, err := charmap.Windows1251.NewEncoder().Bytes([]byte(sample))
	if err != nil {
		t.Fatalf("encode Windows-1251: %v", err)
	}
	if got := DecodeConsoleOutput(win1251Bytes); got != sample {
		t.Fatalf("Windows-1251 decode mismatch: got %q, want %q", got, sample)
	}
}
