package shared

import "testing"

func TestMoneyString(t *testing.T) {
	cases := []struct {
		in   Money
		want string
	}{
		{1050, "R$ 10,50"},
		{5, "R$ 0,05"},
		{0, "R$ 0,00"},
		{-1050, "-R$ 10,50"},
		{120000, "R$ 1200,00"},
	}
	for _, c := range cases {
		if got := c.in.String(); got != c.want {
			t.Errorf("Money(%d).String() = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestApplyBps(t *testing.T) {
	cases := []struct {
		name  string
		gross Money
		rate  Bps
		want  Money
	}{
		{"2,5% de R$ 100,00", 10000, 250, 250},
		{"1,5% de R$ 100,00", 10000, 150, 150},
		{"3,5% de R$ 1.200,00", 120000, 350, 4200},
		{"trunca para baixo", 1001, 250, 25}, // 25,025 -> 25
		{"taxa zero", 10000, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ApplyBps(c.gross, c.rate); got != c.want {
				t.Fatalf("got %d want %d", got, c.want)
			}
		})
	}
}

func TestSplit(t *testing.T) {
	cases := []struct {
		name  string
		total Money
		n     int
		want  []Money
	}{
		{"divide exato", 115800, 12, []Money{9650, 9650, 9650, 9650, 9650, 9650, 9650, 9650, 9650, 9650, 9650, 9650}},
		{"sobra vai para a primeira", 1000, 3, []Money{334, 333, 333}},
		{"uma parcela", 999, 1, []Money{999}},
		{"n inválido", 999, 0, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Split(c.total, c.n)
			if len(got) != len(c.want) {
				t.Fatalf("len = %d want %d", len(got), len(c.want))
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("parte %d = %d want %d", i, got[i], c.want[i])
				}
			}
			if c.n > 0 && Sum(got...) != c.total {
				t.Fatalf("soma %d != total %d", Sum(got...), c.total)
			}
		})
	}
}
