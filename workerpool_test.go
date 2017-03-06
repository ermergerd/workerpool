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

func ExampleWorkerPool_doWithTimeout() {
	p := NewPool(1, false)
	for i := 0; i < 10; i++ {
		p.DoWithTimeout(func() {
			fmt.Printf("Hello %d\n", i)
		}, time.Millisecond)
	}
	p.Close()
}
