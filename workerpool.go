package workerpool

import (
	"sync"
	"time"
)

// A Error is an error type returned by a WorkerPool
type Error string

func (e Error) Error() string {
	return string(e)
}

// ErrClosedWorkerPool is returned when operations are attempted on a closed WorkerPool
const ErrClosedWorkerPool = Error("Worker pool is closed")

// ErrTimedOutWaitingForDo is returned when DoWithTimeout is called and times out waiting
// for a worker to start the requested work
const ErrTimedOutWaitingForDo = Error("No workers available - timed out waiting for worker")

// WorkerPool is a pool of goroutines capable of executing simple functions of type
// func().  The workerpool can be setup to limit the maximum number of workers to
// an integer n and can be setup such that an idle worker is culled at a specified
// interval.
type WorkerPool struct {
	work    chan func()
	limiter chan struct{}

	// This is assumed to be constant after initial construction
	timeout chan struct{}

	wait sync.WaitGroup

	closeonce sync.Once
	close     chan struct{}
}

// NewPool creates a new WorkerPool with a limit of n workers.  If prespawn is
// true, the worker goroutines are prespawned so they are available on the first
// call to Do.
func NewPool(n int, prespawn bool) *WorkerPool {
	p := &WorkerPool{
		work:      make(chan func()),
		limiter:   make(chan struct{}, n),
		timeout:   make(chan struct{}),
		wait:      sync.WaitGroup{},
		closeonce: sync.Once{},
		close:     make(chan struct{}),
	}

	if prespawn {
		// Make a wait group to ensure all the workers get started and pend
		var w sync.WaitGroup
		w.Add(1)
		for i := 0; i < n; i++ {
			p.Do(func() {
				w.Wait()
			})
		}
		w.Done()
	}

	return p
}

// NewPoolWithTimeout creates a new WorkerPool with a limit of n workers and a culling
// interval of t duration.
func NewPoolWithTimeout(n int, t time.Duration) *WorkerPool {
	p := &WorkerPool{
		work:      make(chan func()),
		limiter:   make(chan struct{}, n),
		timeout:   make(chan struct{}),
		wait:      sync.WaitGroup{},
		closeonce: sync.Once{},
		close:     make(chan struct{}),
	}

	// Start the timeout goroutine
	go destroyer(t, p.timeout, p.close)

	return p
}

// Do executes the specified function f using an existing worker if possible
// or a new worker if the worker quota is not yet filled.  The call will
// block if the worker quota is filled and there are no workers available
// until a worker becomes available to do the work or the pool is closed.
func (p *WorkerPool) Do(f func()) error {
	return p.do(f, nil)
}

// DoWithTimeout executes the specified function f using an existing worker if
// possible or a new worker if the worker quota is not yet filled.  The call will
// block if the worker quota is filled and there are no workers available until
// a worker becomes available to do the work, the pool is closed or the specified
// time is reached.
func (p *WorkerPool) DoWithTimeout(f func(), d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()

	return p.do(f, t.C)
}

// do encapsulates the common functionality of Do and DoWithTimeout.  Actually
// sending the work on the work channel to the existing workers and processing
// closure, worker limits, creating new workers, and handling timeout.
func (p *WorkerPool) do(f func(), t <-chan time.Time) error {
	select {
	case p.work <- f:
	case <-p.close:
		return ErrClosedWorkerPool
	default:
		select {
		case p.work <- f:
		case p.limiter <- struct{}{}:
			go func() {
				p.wait.Add(1)
				doWork(f, p.timeout, p.close, p.work)
				<-p.limiter
				p.wait.Done()
			}()
		case <-p.close:
			return ErrClosedWorkerPool
		case <-t:
			return ErrTimedOutWaitingForDo
		}
	}

	return nil
}

// Close closes the worker pool and waits for all workers to complete.
func (p *WorkerPool) Close() {
	p.closeonce.Do(func() {
		close(p.close)
		p.wait.Wait()
	})
}

// destroyer is function that is run as a helper goroutine to periodically
// kill off idle workers at a configurable time interval.  If no worker
// is idle, the routine waits until the next cycle to try again.
func destroyer(t time.Duration, tc chan struct{}, close chan struct{}) {
	tick := time.NewTicker(t)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			select {
			case tc <- struct{}{}:
			case <-close:
				return
			default:
			}
		case <-close:
			return
		}
	}
}

// do is a helper function which is the actual worker goroutine.  It will
// perform an initial unit of work (calling the supplied function) and then
// wait for more work, a timeout, or the closure of the pool.
func doWork(f func(), timeout chan struct{}, close chan struct{}, work chan func()) {
	// Perform the work the was requested
	f()

	// Stick around waiting for additional work
	for {
		// Wait for timeout, close, or more work to do
		select {
		case <-timeout:
			return
		case <-close:
			return
		case f = <-work:
			f()
		}
	}
}
