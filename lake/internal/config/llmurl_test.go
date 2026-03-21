package config

import "testing"

func TestNormalizeOpenAIv1BaseURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"http://localhost:11434/v1", "http://localhost:11434/v1"},
		{"http://localhost:11434/v1/", "http://localhost:11434/v1"},
		{"http://localhost:11434", "http://localhost:11434/v1"},
		{"https://inference.example.com", "https://inference.example.com/v1"},
		{"https://inference.example.com/", "https://inference.example.com/v1"},
		{"https://inference.example.com/v1", "https://inference.example.com/v1"},
	}
	for _, tc := range cases {
		if got := NormalizeOpenAIv1BaseURL(tc.in); got != tc.want {
			t.Errorf("NormalizeOpenAIv1BaseURL(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}
