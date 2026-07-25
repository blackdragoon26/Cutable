package agent

import "testing"

func TestSafePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{"src/App.tsx", "/home/user/react-app/src/App.tsx", true},
		{"/home/user/react-app/src/App.tsx", "/home/user/react-app/src/App.tsx", true},
		{"../secrets", "", false},
		{"/etc/passwd", "/home/user/react-app/etc/passwd", true},
		{"", "", false},
	}
	for _, test := range tests {
		got, err := safePath(test.input)
		if test.ok && (err != nil || got != test.want) {
			t.Errorf("safePath(%q) = %q, %v; want %q", test.input, got, err, test.want)
		}
		if !test.ok && err == nil {
			t.Errorf("safePath(%q) unexpectedly succeeded with %q", test.input, got)
		}
	}
}
