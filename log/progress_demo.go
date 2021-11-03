/*
Copyright © 2021 Opsani <support@opsani.com>
This file is part of https://github.com/opsani/opsani-ignite
*/

package log

import (
	"time"
)

func job(update UpdateFunc) error {
	apps := 0
	for n := 0; n < 5; n++ {
		for a := 0; a < 7; a++ {
			update(ProgressInfo{
				namespacesTotal: 5,
				namespacesDone:  n,
				workloadsTotal:  (n + 1) * 7,
				workloadsDone:   apps,
			})
			apps++
			time.Sleep(325 * time.Millisecond)
		}
	}
	update(ProgressInfo{
		namespacesTotal: 5,
		namespacesDone:  5,
		workloadsTotal:  5 * 7,
		workloadsDone:   apps,
	})
	return nil
}

func DemoProgress() error {
	return GoWithProgress(job)
}
