package math

import "testing"

func getTestCases() []struct{ x, y float64 } {
	return []struct{ x, y float64 }{
		{0, 0},
		{1, 1},
		{11, 11},
		{100, 100},
		{1234, 1234},
		{12345, 12345},
		{123456, 123456},

		{0.1, 0.1},
		{0.33333333, 0.333},
		{0.55555555, 0.556},

		{0.000000123, 0.000000123},
		{0.0000001234, 0.0000001234},
		{0.00000012345, 0.0000001235},

		{1.1, 1.1},
		{1.11, 1.11},
		{1.111, 1.111},
		{1.1111, 1.111},

		{11.1, 11.1},
		{11.11, 11.11},
		{11.111, 11.11},
		{11.1111, 11.11},

		{101.1, 101.1},
		{101.123456, 101.1},
		{101.555555, 101.6},

		{1010.1, 1010},
		{1010.123456, 1010},
		{1010.555555, 1011},

		//TODO: add tests with negative numbers
	}
}

func TestRounding(t *testing.T) {
	for _, c := range getTestCases() {
		res := MagicRound(c.x)
		if res != c.y {
			t.Errorf("MagicRound(%g) should return %g, got %g", c.x, c.y, res)
		}
	}
}
