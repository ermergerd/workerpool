// Package workerpool provides a basic pool of goroutine workers
// that can execute an arbitrary simple function of the type func().  The
// pool can be used to perform tasks that may block for an indeterminate
// amount of time such as writing data to a network connection, sending on
// a channel which is only serviced by goroutines that may be doing network
// connection writing, etc.
package workerpool
