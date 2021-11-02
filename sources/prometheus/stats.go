/*
Copyright © 2021 Opsani <support@opsani.com>
This file is part of https://github.com/opsani/opsani-ignite
*/

package prometheus

import (
	"fmt"
	"math"
	"sort"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"opsani-ignite/log"
	opsmath "opsani-ignite/math"
)

type ValueStats struct {
	N       int
	Min     float64
	Max     float64
	Sum     float64
	Average float64
	Median  float64
	StDev   float64
}

func calcSamplePairStats(samples []model.SamplePair) (res ValueStats) {
	// handle the no-data case (Min, Max are not valid if N==0)
	if len(samples) == 0 {
		return
	}

	values := make([]float64, 0, len(samples))

	// accumulate min, max and sum
	first := float64(samples[0].Value)
	res.N = 1
	res.Min = first
	res.Max = first
	res.Sum = first
	values = append(values, first)
	for _, s := range samples[1:] {
		v := float64(s.Value)
		values = append(values, v)
		res.Sum += v
		res.N += 1
		if v < res.Min {
			res.Min = v
		}
		if v > res.Max {
			res.Max = v
		}
	}

	// compute average
	res.Average = res.Sum / float64(res.N) // N != 0 always

	// compute median
	sort.Float64s(values)
	if res.N%2 == 1 {
		res.Median = values[(res.N+1)/2-1]
	} else {
		res.Median = (values[res.N/2-1] + values[res.N/2]) / 2
	}

	// compute standard deviation
	acc := 0.0
	for _, v := range values {
		acc += math.Pow(v-res.Average, 2)
	}
	res.StDev = math.Sqrt(acc / float64(res.N-1))

	return
}

func valueFromSamplePairs(samples []model.SamplePair, logLabel string) (float64, v1.Warnings, error) {
	if len(samples) == 0 {
		return math.NaN(), nil, fmt.Errorf("No samples in data series for %v", logLabel)
	}

	// compute statistics for the data series
	d := calcSamplePairStats(samples)
	if logLabel != "" {
		log.Tracef("Series statistics for %v: %#v", logLabel, d)
	}

	//TODO look through data series, indicate warnings for uneven distribution, gaps, etc.
	var warnings v1.Warnings

	// average != median typically indicates uneven distribution
	if math.Abs(d.Average-d.Median) > 0.1*d.Average {
		warnings = append(warnings, fmt.Sprintf("Potentially uneven distribution for %v: average %v, median %v", logLabel, d.Average, d.Median))
	}
	if d.Average != 0.0 && d.StDev/d.Average > 0.1 {
		warnings = append(warnings, fmt.Sprintf("Potentially uneven distribution for %v: average %v, stdev %v", logLabel, d.Average, d.StDev))
	}

	// choose value to return (TODO depending on distribution, use Average, Median, Max, etc.)
	v := opsmath.MagicRound(d.Average)

	return v, warnings, nil
}
