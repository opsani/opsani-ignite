/*
Copyright © 2021 Opsani <support@opsani.com>
This file is part of https://github.com/opsani/opsani-ignite
*/

package log

import (
	"time"
)

func job_abs(update ProgressUpdateFunc) error {
	apps := 0
	for n := 0; n < 5; n++ {
		for a := 0; a < 7; a++ {
			update(ProgressInfo{
				NamespacesTotal: 5,
				NamespacesDone:  n,
				WorkloadsTotal:  (n + 1) * 7,
				WorkloadsDone:   apps,
			}, false)
			apps++
			time.Sleep(325 * time.Millisecond)
		}
	}
	update(ProgressInfo{
		NamespacesTotal: 5,
		NamespacesDone:  5,
		WorkloadsTotal:  5 * 7,
		WorkloadsDone:   apps,
	}, false)
	return nil
}

func job_rel(update ProgressUpdateFunc) error {
	update(ProgressInfo{
		NamespacesTotal: 5,
		NamespacesDone:  0,
		WorkloadsTotal:  5 * 7,
		WorkloadsDone:   0,
	}, false)
	for n := 0; n < 5; n++ {
		for a := 0; a < 7; a++ {
			update(ProgressInfo{WorkloadsDone: 1}, true)
			time.Sleep(325 * time.Millisecond)
		}
		update(ProgressInfo{NamespacesDone: 1}, true)
	}
	return nil
}


func DemoProgress() error {
	if err := GoWithProgress(job_abs); err != nil {
		return err
	}
	if err := GoWithProgress(job_rel); err != nil {
		return err
	}
	return nil
}
