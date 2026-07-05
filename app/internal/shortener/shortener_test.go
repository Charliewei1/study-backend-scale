package shortener

import "testing"

func TestShortenerNextFormat(t *testing.T) {
	s := New()

	code, err := s.Next()
	if err != nil {
		t.Fatalf("Next returned error: %v", err)
	}

	if len(code) != codeLength {
		t.Fatalf("code length = %d, want %d; code=%q", len(code), codeLength, code)
	}
	assertBase62Code(t, code)
}

func TestShortenerNextUniqueAcrossManyGenerations(t *testing.T) {
	s := New()
	seen := make(map[string]struct{}, 10_000)

	for i := 0; i < 10_000; i++ {
		code, err := s.Next()
		if err != nil {
			t.Fatalf("Next returned error on iteration %d: %v", i, err)
		}
		assertBase62Code(t, code)
		if _, ok := seen[code]; ok {
			t.Fatalf("duplicate code generated on iteration %d: %q", i, code)
		}
		seen[code] = struct{}{}
	}
}

func assertBase62Code(t *testing.T, code string) {
	t.Helper()

	for _, ch := range code {
		switch {
		case '0' <= ch && ch <= '9':
		case 'a' <= ch && ch <= 'z':
		case 'A' <= ch && ch <= 'Z':
		default:
			t.Fatalf("code %q contains non-base62 character %q", code, ch)
		}
	}
}
