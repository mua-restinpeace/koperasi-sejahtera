package helper

import "testing"

func TestInisial(t *testing.T) {
	tests := []struct{ nama, want string }{
		{"Budi Pratama", "BP"},
		{"  siti   rahmawati  dewi ", "SR"},
		{"Admin", "A"},
		{"", "?"},
		{"   ", "?"},
		{"123 !!", "?"},
	}
	for _, tt := range tests {
		if got := Inisial(tt.nama); got != tt.want {
			t.Errorf("Inisial(%q) = %q, want %q", tt.nama, got, tt.want)
		}
	}
}
