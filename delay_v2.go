package delay

import (
	"sync"
	"time"
)

// NOIE !!! delayer can not run any dead loop func
// Compare to V1, V2 will use less goroutine even less memory
type DelayFunc func(arg interface{})

type DelayerV2 struct {
	fn DelayFunc

	cancel chan bool

	active       bool
	buffer_size  int64
	minDelayTime int64 // ms
	maxDelayTime int64 // ms

	buffer  []interface{}
	pending chan []interface{}

	lk sync.Mutex
	wg sync.WaitGroup
	// consider if use grpool or not
}

func NewDelayV2(fn DelayFunc, bufferSize, minTime, maxTime int64) *DelayerV2 {
	dl := new(DelayerV2)
	dl.fn = fn
	dl.active = false
	dl.buffer_size = bufferSize
	dl.minDelayTime = minTime
	dl.maxDelayTime = maxTime

	dl.buffer = make([]interface{}, 0, bufferSize)
	dl.pending = make(chan []interface{}, (maxTime/(maxTime-minTime))+5)

	dl.cancel = make(chan bool, 1)

	return dl
}

func (dl *DelayerV2) Start() {
	// start two goroutine, one for copy data to pending process
	// another one for processing the ready data
	go func() {
		bfT := time.NewTicker(time.Duration(dl.maxDelayTime - dl.minDelayTime) * time.Millisecond)
		for {
			select {
			case <-dl.cancel:
				dl.cancel <- true
				return
			case <-bfT.C:
				dl.lk.Lock()
				copys := make([]interface{}, 0, len(dl.buffer))
				copys = append(copys, dl.buffer...)
				dl.pending <- copys
				dl.buffer = dl.buffer[:0]
				dl.lk.Unlock()
			}
		}
	}()
	go func() {
		for {
			select {
			case <-dl.cancel:
				dl.cancel <- true
				return
			case pending := <- dl.pending:
				if len(pending) > 0 {
					dl.wg.Add(1)
					go dl.doPending(pending)
				}
			}
		}
	}()
	dl.active = true
}

func (dl *DelayerV2) Stop() {
	dl.active = false
	dl.cancel <- true
	// wait for a moment
	time.Sleep(time.Millisecond * 50)
	<- dl.cancel

	dl.Flush()

	dl.wg.Wait()
}

func (dl *DelayerV2) Exec(arg interface{}) {
	if !dl.active {
		return
	}
	dl.lk.Lock()
	defer dl.lk.Unlock()

	if int64(len(dl.buffer)) >= dl.buffer_size {
		return
	}

	dl.buffer = append(dl.buffer, arg)
}

func (dl *DelayerV2) doPending(pending []interface{}) {
	defer dl.wg.Done()
	time.Sleep(time.Duration(dl.minDelayTime) * time.Millisecond)
	for _, item := range pending {
		dl.fn(item)
	}
}

func (dl *DelayerV2) Flush() {
	dl.lk.Lock()

	copys := make([]interface{}, 0, len(dl.buffer))
	copys = append(copys, dl.buffer...)
	dl.buffer = dl.buffer[:0]

	dl.lk.Unlock()
	for _, item := range copys {
		dl.fn(item)
	}
}
