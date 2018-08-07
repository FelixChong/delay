package delay

import (
	"sync"
	"time"
	)

type Delayer struct {
	After    time.Duration
	Callback func(args ...interface{})
	timers   map[string]*time.Timer
	maxproc  int

	wg     sync.WaitGroup
	active bool
}

func NewDelayer(fn func(args ...interface{}), after time.Duration, maxproc int) *Delayer {
	delayer := &Delayer{After: after, Callback: fn}
	delayer.timers = make(map[string]*time.Timer)
	delayer.maxproc = maxproc
	delayer.active = true
	return delayer
}

func (d *Delayer) Register(key string, args ...interface{}) {
	if !d.active {
		return
	}
	if len(d.timers) >= d.maxproc {
		return
	}

	if _, ok := d.timers[key]; ok {
		d.Cancel(key)
	}

	// override directly
	d.wg.Add(1)
	d.timers[key] = time.AfterFunc(d.After, func() {
		defer func() {
			delete(d.timers, key)
			d.wg.Done()
		}()
		d.Callback(args...)
	})
}

func (d *Delayer) Cancel(key string) bool {
	timer, ok := d.timers[key]
	if !ok {
		return false
	}

	timer.Stop()
	delete(d.timers, key)
	// cancel should treat task done as well
	d.wg.Done()
	return true
}

func (d *Delayer) Pending() int {
	return len(d.timers)
}

func (d *Delayer) Flush(keys ...string) int {
	var timers map[string]*time.Timer

	if len(keys) > 0 {
		timers = make(map[string]*time.Timer)
		for _, key := range keys {
			if timer, ok := d.timers[key]; ok {
				timers[key] = timer
			}
		}
	} else {
		// If no keys are specified, flush all of them
		timers = d.timers
	}

	flushed := 0
	for _, timer := range timers {
		if timer.Reset(0) == true {
			flushed = flushed + 1
		}
	}

	return flushed
}

func (d *Delayer) Stop() {
	d.active = false
	d.Flush()
	d.wg.Wait()
}