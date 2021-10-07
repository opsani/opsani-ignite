/*
Copyright © 2021 Opsani <support@opsani.com>
This file is part of https://github.com/opsani/opsani-ignite
*/

package math

import (
	m "math"
)

func Min(samples ...float64) float64 {
	min := m.NaN()
	for _, val := range samples {
		if m.IsNaN(val) || m.IsInf(val, 0) {
			continue
		}
		if m.IsNaN(min) || val < min {
			min = val
		}
	}
	return min // will return NaN for empty slice or slice that has no valid values
}

func Sum(samples ...float64) float64 {
	total := 0.0
	for _, val := range samples {
		if m.IsNaN(val) || m.IsInf(val, 0) {
			continue
		}
		total += val
	}
	return total
}

func Avg(samples ...float64) float64 {
	total := 0.0
	count := 0
	for _, val := range samples {
		if m.IsNaN(val) || m.IsInf(val, 0) {
			continue
		}
		total += val
		count += 1
	}

	if count == 0 {
		return 0.0
	}
	return total / float64(len(samples))
}
