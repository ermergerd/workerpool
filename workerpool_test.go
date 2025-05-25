package workerpool

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestWorkerPoolN10Do(t *testing.T) {
	p := NewPool(10, false)

	// We need wait groups to wait for the start and completion of our workers
	ws := sync.WaitGroup{}
	we := sync.WaitGroup{}

	// Add one blocking function to the pool (this should allocate a single worker)
	c := make(chan struct{})
	ws.Add(1)
	err := p.Do(func() {
		we.Add(1)
		ws.Done()
		<-c
		we.Done()
	})
	if err != nil {
		t.Fatal("Unexpected error when queueing work")
	}

	// Make sure the work has started
	ws.Wait()
	if len(p.limiter) != 1 {
		t.Fatal("Limiter should have 1 entry")
	}

	// Add 9 more jobs
	c2 := make(chan struct{})
	for i := 0; i < 9; i++ {
		ws.Add(1)
		err = p.Do(func() {
			we.Add(1)
			ws.Done()
			<-c2
			we.Done()
		})
		if err != nil {
			t.Fatal("Unexpected error when queueing work")
		}
	}

	// Make sure the work has started
	ws.Wait()
	if len(p.limiter) != 10 {
		t.Fatal("Limiter should have 10 entries")
	}

	// Attempt to send work to the pool (no workers should be available and Do should block)
	sc := make(chan struct{})
	done := make(chan struct{})
	go func() {
		p.Do(func() {
			we.Add(1)
			sc <- struct{}{}
			<-c2
			we.Done()
		})
		close(done)
	}()

	dt := time.NewTimer(250 * time.Millisecond)
	select {
	case <-done:
		dt.Stop()
		t.Fatal("Should not have been able to queue work - not enough workers")
	case <-sc:
		dt.Stop()
		t.Fatal("Work should not have started - not enough workers")
	case <-dt.C:
	}

	// Complete the first job so the pending one can run
	close(c)
	dt.Reset(1 * time.Second)
	select {
	case <-sc:
		dt.Stop()
	case <-dt.C:
		t.Fatal("The work should have been picked up by a worker")
	}
	if len(p.limiter) != 10 {
		t.Fatal("Limiter should have 10 entries")
	}

	// Complete the rest of the work and let the workers finish
	close(c2)
	we.Wait()

	// Since we didn't specify a timeout, the workers should still exist
	if len(p.limiter) != 10 {
		t.Fatal("Limiter should have 10 entries")
	}

	// But we should be able to throw more work to the pool now
	c3 := make(chan struct{})
	for i := 0; i < 10; i++ {
		ws.Add(1)
		err = p.Do(func() {
			we.Add(1)
			ws.Done()
			<-c3
			we.Done()
		})
		if err != nil {
			t.Fatal("Unexpected error when queueing work")
		}
	}

	// Wait for the work to start
	ws.Wait()
	if len(p.work) != 0 {
		t.Fatal("Work not yet started for some jobs (workers should be available)")
	}

	// Finish the work and wait for the work to be complete
	close(c3)
	we.Wait()

	// Now close the pool and ensure that everything gets cleaned up
	p.Close()
	if len(p.limiter) != 0 {
		t.Fatal("Limiter should have 0 entries")
	}
}

func TestWorkerPoolN10DoWithTimeout(t *testing.T) {
	p := NewPool(10, false)

	// We need wait groups to wait for start and completion of our workers
	ws := sync.WaitGroup{}
	we := sync.WaitGroup{}

	// Fill up the worker pool with blocked workers
	c := make(chan struct{})
	for i := 0; i < 10; i++ {
		ws.Add(1)
		err := p.Do(func() {
			we.Add(1)
			ws.Done()
			<-c
			we.Done()
		})
		if err != nil {
			t.Fatal("Unexpected error when queueing work")
		}
	}

	// Wait for the workers to start
	ws.Wait()
	if len(p.limiter) != 10 {
		t.Fatal("Limiter should have 10 entries")
	}

	// Now try to send work with a timeout (timeout should get hit)
	err := p.DoWithTimeout(func() {
		we.Add(1)
		<-c
		we.Done()
	}, 250*time.Millisecond)
	if err != ErrTimedOutWaitingForDo {
		t.Fatal("Expected timeout error but got: ", err)
	}

	// Finish the work and wait for the workers
	close(c)
	we.Wait()
	if len(p.limiter) != 10 {
		t.Fatal("Limiter should have 10 entries")
	}

	// Close the workerpool and check that things are cleaned up
	p.Close()
	if len(p.limiter) != 0 {
		t.Fatal("Limiter should have 0 entries (workerpool is closed)")
	}

	// Check that we get closed errors
	err = p.Do(func() {})
	if err != ErrClosedWorkerPool {
		t.Fatal("Workerpool should have returned a closed error")
	}
	t.Log(err)
}

func TestWorkerPoolN10Primed(t *testing.T) {
	p := NewPool(10, true)

	// The primed worker pool should be maxed out at 10
	if len(p.limiter) != 10 {
		t.Fatal("Limiter should have 10 entries: ", len(p.limiter))
	}

	// We need wait groups to wait for the start and completion of our workers
	ws := sync.WaitGroup{}
	we := sync.WaitGroup{}

	// Add one blocking function to the pool (this should allocate a single worker)
	c := make(chan struct{})
	ws.Add(1)
	err := p.Do(func() {
		we.Add(1)
		ws.Done()
		<-c
		we.Done()
	})
	if err != nil {
		t.Fatal("Unexpected error when queueing work")
	}

	// Make sure the work has started and check that there are still 10 workers
	ws.Wait()
	if len(p.limiter) != 10 {
		t.Fatal("Limiter should have 10 entries")
	}

	// Add 9 more jobs
	for i := 0; i < 9; i++ {
		ws.Add(1)
		err = p.Do(func() {
			we.Add(1)
			ws.Done()
			<-c
			we.Done()
		})
		if err != nil {
			t.Fatal("Unexpected error when queueing work")
		}
	}

	// Make sure the work has started and check that there are still 10 workers
	ws.Wait()
	if len(p.limiter) != 10 {
		t.Fatal("Limiter should have 10 entries")
	}

	// Finish the work and wait for the workers
	close(c)
	we.Wait()
	if len(p.limiter) != 10 {
		t.Fatal("Limiter should have 10 entries")
	}

	// Close the workerpool and check that things are cleaned up
	p.Close()
	if len(p.limiter) != 0 {
		t.Fatal("Limiter should have 0 entries (workerpool is closed)")
	}
}

func TestWorkerPoolWithTimeoutN5Do(t *testing.T) {
	p := NewPoolWithTimeout(5, 100*time.Millisecond)

	// We need wait groups to wait for start and completion of our workers
	ws := sync.WaitGroup{}
	we := sync.WaitGroup{}

	// Fill up the worker pool with blocked workers
	c := make(chan struct{})
	for i := 0; i < 5; i++ {
		ws.Add(1)
		err := p.Do(func() {
			we.Add(1)
			ws.Done()
			<-c
			we.Done()
		})
		if err != nil {
			t.Fatal("Unexpected error when queueing work")
		}
	}

	// Wait for the workers to start
	ws.Wait()
	if len(p.limiter) != 5 {
		t.Fatal("Limiter should have 5 entries")
	}

	// Now complete the work
	close(c)
	we.Wait()

	// Make sure some workers still exist after 100ms
	time.Sleep(100 * time.Millisecond)
	if len(p.limiter) == 0 {
		t.Fatal("Not all workers should be killed yet")
	}

	// The workers should start getting culled (it should ideally take 5 * 100ms = 500ms, we will wait 600ms)
	time.Sleep(500 * time.Millisecond)
	if len(p.limiter) != 0 {
		t.Fatal("Limiter should have 0 entries")
	}

	// Add some work to the worker pool so we have something to clean up
	c2 := make(chan struct{})
	ws.Add(1)
	p.Do(func() {
		we.Add(1)
		ws.Done()
		<-c2
		we.Done()
	})

	// Wait for the worker to start
	ws.Wait()
	if len(p.limiter) != 1 {
		t.Fatal("Limiter should have 1 entry")
	}

	// Complete the work
	close(c2)
	we.Wait()

	// Close the pool and check that things are cleaned up
	p.Close()
	if len(p.limiter) != 0 {
		t.Fatal("Limiter should have 0 entries after close: ", len(p.limiter))
	}
}

func TestWorkerPoolClose(t *testing.T) {
	// Make a pool with a destroyer timeout that is very slow (try to catch the close)
	p := NewPoolWithTimeout(1, 1*time.Millisecond)

	// We need wait groups to wait for start and completion of our workers
	ws := sync.WaitGroup{}
	we := sync.WaitGroup{}

	// Fill up the pool with a blocked worker
	c := make(chan struct{})
	ws.Add(1)
	p.Do(func() {
		we.Add(1)
		ws.Done()
		<-c
		time.Sleep(100 * time.Millisecond)
		we.Done()
	})
	ws.Wait()

	// Now attempt to put more work into the queue
	c2 := make(chan struct{})
	ws.Add(1)
	go func() {
		we.Add(1)
		ws.Done()
		err := p.Do(func() {
			we.Add(1)
			<-c2
			we.Done()
		})
		if err != ErrClosedWorkerPool {
			t.Fatal("Do should return closed error when worker pool closed during blocking")
		}
		we.Done()
	}()
	ws.Wait()

	// Now close the pool and wait for everything to finish
	close(c)
	p.Close()
	we.Wait()

	// Since we didn't close c2, we could also validate that there aren't any workers in flight
	if len(p.limiter) != 0 {
		t.Fatal("Limiter should have 0 entries")
	}
}

func ExampleWorkerPool_do() {
	p := NewPool(10, true)
	for i := 0; i < 10; i++ {
		p.Do(func() {
			fmt.Printf("Hello %d\n", i)
		})
	}
	p.Close()
}

// expectPanic is a helper function to assert that a function call panics.
func expectPanic(t *testing.T, f func(), msg string) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("%s: expected a panic but did not get one", msg)
		}
	}()
	f()
}

func TestWorkerPoolZeroAndNegativeN(t *testing.T) {
	t.Run("n=0", func(t *testing.T) {
		p := NewPool(0, false)
		err := p.DoWithTimeout(func() {}, 50*time.Millisecond)
		if err != ErrTimedOutWaitingForDo {
			t.Errorf("Expected ErrTimedOutWaitingForDo, got %v", err)
		}
		p.Close()
	})

	t.Run("n=-1", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("NewPool(-1) should have panicked")
			}
		}()
		// This call is expected to panic because make(chan struct{}, n) panics for n < 0
		_ = NewPool(-1, false)
		// If NewPool(-1) does not panic, we might want to close it if it were returned,
		// but the test should fail before this.
		// p.Close() // This line would only be reached if NewPool(-1) didn't panic.
	})
}

func TestWorkerPoolDoNilFunction(t *testing.T) {
	t.Run("Do_nil_function", func(t *testing.T) {
		// Set GOMAXPROCS to 1 to make the scheduling more predictable for this test.
		// This is not strictly necessary but can help in avoiding flakiness
		// if the test environment has multiple cores and things happen too fast.
		// Note: Changing GOMAXPROCS might affect other parallel tests if not restored.
		// For this specific test, the impact should be minimal as it's self-contained.

		p := NewPool(1, false)
		if p == nil {
			t.Fatal("NewPool returned nil")
		}

		// Dispatch a nil function. We expect the worker goroutine to panic.
		// The Do method itself doesn't return an error for a nil func.
		// The panic will occur in the goroutine started by p.run().
		_ = p.Do(nil)

		// Allow time for the worker goroutine to pick up the nil job and panic.
		time.Sleep(100 * time.Millisecond) // Increased sleep time for more reliability

		// Now, try to dispatch a valid job.
		// If the first (and only) worker panicked and did not properly release
		// its slot in p.limiter, this new job should not find an available worker.
		err := p.DoWithTimeout(func() {
			// This is a valid job that should execute if a worker was available.
			t.Log("Valid job in Do_nil_function executed unexpectedly")
		}, 50*time.Millisecond)

		if err != ErrTimedOutWaitingForDo {
			t.Errorf("Expected ErrTimedOutWaitingForDo after nil func in Do, got %v", err)
		}
		p.Close()
	})

	t.Run("DoWithTimeout_nil_function", func(t *testing.T) {
		p2 := NewPool(1, false)
		if p2 == nil {
			t.Fatal("NewPool returned nil")
		}

		// Dispatch a nil function using DoWithTimeout.
		// DoWithTimeout itself should queue the job. The panic happens in the worker.
		// The error from this call would typically be ErrTimedOutWaitingForDo if the pool was busy,
		// or nil if successfully queued.
		errDispatch := p2.DoWithTimeout(nil, 50*time.Millisecond)
		if errDispatch != nil && errDispatch != ErrTimedOutWaitingForDo {
			// This case means DoWithTimeout itself had an issue other than timeout,
			// which is not the primary focus, but good to note.
			t.Logf("Initial DoWithTimeout(nil, ...) returned: %v", errDispatch)
		}


		// Allow time for the worker goroutine to pick up the nil job and panic.
		time.Sleep(100 * time.Millisecond) // Increased sleep time

		// Try to dispatch another valid job.
		// Similar to the Do case, if the worker panicked and didn't release the limiter,
		// this should time out.
		err := p2.DoWithTimeout(func() {
			// This is a valid job.
			t.Log("Valid job in DoWithTimeout_nil_function executed unexpectedly")
		}, 50*time.Millisecond)

		if err != ErrTimedOutWaitingForDo {
			t.Errorf("Expected ErrTimedOutWaitingForDo after nil func in DoWithTimeout, got %v", err)
		}
		p2.Close()
	})
}

func TestWorkerPoolWithTimeoutZeroAndNegativeN(t *testing.T) {
	t.Run("n=0", func(t *testing.T) {
		p := NewPoolWithTimeout(0, 100*time.Millisecond)
		err := p.DoWithTimeout(func() {}, 50*time.Millisecond)
		if err != ErrTimedOutWaitingForDo {
			t.Errorf("Expected ErrTimedOutWaitingForDo, got %v", err)
		}
		p.Close()
	})

	t.Run("n=-1", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("NewPoolWithTimeout(-1, ...) should have panicked")
			}
		}()
		// This call is expected to panic because make(chan struct{}, n) panics for n < 0
		_ = NewPoolWithTimeout(-1, 100*time.Millisecond)
		// If NewPoolWithTimeout(-1, ...) does not panic, we might want to close it if it were returned,
		// but the test should fail before this.
	})
}

func TestWorkerPoolWithTimeoutInvalidTimeout(t *testing.T) {
	t.Run("zero_timeout", func(t *testing.T) {
		expectPanic(t, func() {
			// We expect NewPoolWithTimeout to panic because the destroyer function
			// inside it will try to create a time.NewTicker with a zero duration,
			// which causes a panic.
			p := NewPoolWithTimeout(1, 0*time.Second)
			// If it doesn't panic (which would be a bug in our expectation or the code),
			// we should clean up if a pool was somehow returned.
			if p != nil {
				p.Close()
			}
		}, "NewPoolWithTimeout(1, 0*time.Second)")
	})

	t.Run("negative_timeout", func(t *testing.T) {
		expectPanic(t, func() {
			// Similar to the zero duration case, a negative duration will cause
			// time.NewTicker to panic.
			p := NewPoolWithTimeout(1, -1*time.Second)
			// Cleanup if no panic occurs.
			if p != nil {
				p.Close()
			}
		}, "NewPoolWithTimeout(1, -1*time.Second)")
	})
}

func ExampleWorkerPool_doWithTimeout() {
	p := NewPool(1, false)
	for i := 0; i < 10; i++ {
		p.DoWithTimeout(func() {
			fmt.Printf("Hello %d\n", i)
		}, time.Millisecond)
	}
	p.Close()
}
