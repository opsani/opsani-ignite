/*
Copyright © 2021 Opsani <support@opsani.com>
This file is part of https://github.com/opsani/opsani-ignite
*/

package log

import (
	"fmt"
	"math"
	"os"
	"sync"
	"time"
)

type ProgressInfo struct {
	namespaces int
	workloads  int
}

type UpdateFunc func(info ProgressInfo)
type RunnerFunc func(infoCallback UpdateFunc) error

type progressState struct {
	lock sync.Mutex
	info ProgressInfo
}

func (s *progressState) updateInfo(info ProgressInfo) {
	s.lock.Lock()
	s.info = info
	s.lock.Unlock()
}

func (s *progressState) renderProgress(startTime time.Time, final bool) {
	// safely grab a copy of the info
	s.lock.Lock()
	info := s.info
	s.lock.Unlock()

	// display info as progress
	now := time.Now()
	elapsed := math.Round(now.Sub(startTime).Seconds()*10) / 10
	fmt.Fprintf(os.Stderr, "\rCollecting data (%.1fs): %v namespace(s), %v application(s)... ", elapsed, info.namespaces, info.workloads)
	if final {
		fmt.Fprintf(os.Stderr, "done.\n\n")
	}
}

func GoWithProgress(runner RunnerFunc) error {
	done := make(chan error)
	state := progressState{}
	state.renderProgress(time.Now(), false)

	// run the runner function and notify when done
	go func() {
		err := runner(func(info ProgressInfo) { state.updateInfo(info) })
		done <- err
	}()

	startTime := time.Now()
	var runnerError error
loop:
	for {
		select {
		case err := <-done:
			runnerError = err
			break loop
		default:
			state.renderProgress(startTime, false)
			time.Sleep(time.Duration(100 * time.Millisecond))
		}
	}
	close(done)
	state.renderProgress(startTime, true)

	return runnerError
}
