package lyrics

import "testing"

func TestParseLRC(t *testing.T) {
	data := []byte("[ti:Titulo]\n[ar:Artista]\n[00:12.34]Primeira linha\n[01:02.5]Segunda linha\n[02:00]Terceira\n[03:30.100]Quarta\nlinha sem timestamp\n[offset:+500]\n[04:00]Depois do offset\n")
	lines := ParseLRC(data)
	if len(lines) != 6 {
		t.Fatalf("esperava 6 linhas, got %d: %+v", len(lines), lines)
	}
	want := []struct {
		text  string
		start int64
		has   bool
	}{
		{"Primeira linha", 12340, true},
		{"Segunda linha", 62500, true},
		{"Terceira", 120000, true},
		{"Quarta", 210100, true},
		{"linha sem timestamp", 0, false},
		{"Depois do offset", 240500, true},
	}
	for i, w := range want {
		if lines[i].Text != w.text {
			t.Fatalf("linha %d: texto %q, want %q", i, lines[i].Text, w.text)
		}
		if w.has {
			if lines[i].Start == nil || *lines[i].Start != w.start {
				t.Fatalf("linha %d: start %v, want %d", i, lines[i].Start, w.start)
			}
		} else if lines[i].Start != nil {
			t.Fatalf("linha %d: start inesperado %d", i, *lines[i].Start)
		}
	}
}

func TestParseLRCEmpty(t *testing.T) {
	if lines := ParseLRC([]byte("")); len(lines) != 0 {
		t.Fatalf("esperava vazio, got %+v", lines)
	}
}
