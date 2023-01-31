// Package net provides net related functions
package net

import "fmt"

// FakeReader is a fake reader for the test purpose
type FakeReader struct {
	ExpectErr error
}

// Read is a fake method
func (e *FakeReader) Read(p []byte) (n int, err error) {
	err = e.ExpectErr
	fmt.Println(err, "fake")
	return
}
