package math

import (
	m "math"
)

// MagicRound rounds to whole numbers or up to 4 significant digits
func MagicRound(x float64) float64 {
	if x == 0 {
		return 0
	}
	magnitudeDigits := m.Max(0.0, m.Round(3-m.Log10(x)))
	magnitudeScale := m.Round(m.Pow(10, magnitudeDigits))
	if magnitudeScale == 0 {
		return x // prevents division by 0, although this should never happen
	}
	//fmt.Println(x, magnitudeDigits, magnitudeScale)
	return m.Round(x*magnitudeScale) / magnitudeScale
}
