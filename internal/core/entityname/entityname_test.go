package entityname

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Donald Trump's", "Donald Trump"},
		{"Donald Trump’s", "Donald Trump"}, // curly apostrophe
		{"Congress'", "Congress"},
		{"Donald Trump", "Donald Trump"},
		{"  Elon Musk  ", "Elon Musk"},
		{"", ""},
		{"'s", ""},
	}
	for _, tt := range tests {
		if got := Normalize(tt.in); got != tt.want {
			t.Errorf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
