package shortener

import "testing"

func TestEncode(t *testing.T) {
	tests := []struct {
		name string
		id   uint64
		want string
	}{
		{name: "zero", id: 0, want: "0"},
		{name: "single digit", id: 1, want: "1"},
		{name: "last one digit", id: 61, want: "Z"},
		{name: "first two digits", id: 62, want: "10"},
		{name: "two digits plus one", id: 63, want: "11"},
		{name: "three digits", id: 62 * 62, want: "100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Encode(tt.id); got != tt.want {
				t.Fatalf("Encode(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestShortenerNext(t *testing.T) {
	s := New()

	tests := []struct {
		name string
		want string
	}{
		{name: "first", want: "1"},
		{name: "second", want: "2"},
		{name: "third", want: "3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.Next(); got != tt.want {
				t.Fatalf("Next() = %q, want %q", got, tt.want)
			}
		})
	}
}
