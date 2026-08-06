package phone

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"(11) 98765-4321", "11987654321", false},
		{"11987654321", "11987654321", false},
		{"(99) 99999-9999", "99999999999", false},
		{" 21 99876-5432 ", "21998765432", false},
		{"(11) 9876-5432", "1198765432", false},
		{"", "", true},
		{"12345", "", true},
		{"(11) 98765-4321 0", "", true}, // 12 digits
		{"abc123", "", true},            // too short after stripping
		{"(00) 12345-6789", "", true},   // starts with 0
	}
	for _, c := range cases {
		got, err := Normalize(c.in)
		if c.wantErr {
			if err == nil {
				t.Fatalf("Normalize(%q): expected error, got %q", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("Normalize(%q): unexpected error %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormat(t *testing.T) {
	if got := Format("11987654321"); got != "(11) 98765-4321" {
		t.Fatalf("Format 11 = %q", got)
	}
	if got := Format("2198765432"); got != "(21) 9876-5432" {
		t.Fatalf("Format 10 = %q", got)
	}
}

func TestMask(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"1", "(1"},
		{"11", "(11)"},
		{"1198765", "(11) 98765"},
		{"11987654321", "(11) 98765-4321"},
		{"1198765432", "(11) 9876-5432"},
		{"(11) 98765-4321", "(11) 98765-4321"},
	}
	for _, c := range cases {
		if got := Mask(c.in); got != c.want {
			t.Fatalf("Mask(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
