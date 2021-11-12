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

// ProgressInfo holds the values used to show progress
type ProgressInfo struct {
	NamespacesTotal int
	NamespacesDone  int
	WorkloadsTotal  int
	WorkloadsDone   int
}

// ProgressUpdateFunc is the signature of the progress update callback function.
// The update may set absolute or relative values (relative values are +=).
type ProgressUpdateFunc func(info ProgressInfo, relative bool)

// RunnerFunc is the signature for the function to run with GoWithProgress
type RunnerFunc func(infoCallback ProgressUpdateFunc) error

type progressState struct {
	lock sync.Mutex
	info ProgressInfo
}

func (s *progressState) updateInfo(info ProgressInfo, relative bool) {
	s.lock.Lock()
	if relative {
		s.info.NamespacesTotal += info.NamespacesTotal
		s.info.NamespacesDone  += info.NamespacesDone
		s.info.WorkloadsTotal  += info.WorkloadsTotal
		s.info.WorkloadsDone   += info.WorkloadsDone
	} else {
		s.info = info
	}
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
	fmt.Fprintf(os.Stderr, "\rCollecting data (%.1fs): %v of %v namespace(s) and %v of %v application(s) completed... ",
		elapsed, info.NamespacesDone, info.NamespacesTotal, info.WorkloadsDone, info.WorkloadsTotal)
	if final {
		fmt.Fprintf(os.Stderr, "done.\n\n")
	}
}

// GoWithProgress executes the given runner function as a goroutine while showing progress
func GoWithProgress(runner RunnerFunc) error {
	done := make(chan error)
	state := progressState{}
	state.renderProgress(time.Now(), false)

	// run the runner function and notify when done
	go func() {
		err := runner(func(info ProgressInfo, relative bool) { state.updateInfo(info, relative) })
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
