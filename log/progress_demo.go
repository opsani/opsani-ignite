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
			update(ProgressInfo{namespaces: n, workloads: apps})
			apps++
			time.Sleep(325 * time.Millisecond)
		}
	}
	return nil
}

func DemoProgress() error {
	return GoWithProgress(job)
}
