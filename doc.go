// Package container provides generic, non-concurrent containers.
//
// The container types in this package are not safe for concurrent mutation.
// Callers must provide synchronization when a container is accessed by
// multiple goroutines and at least one access mutates it.
package container
