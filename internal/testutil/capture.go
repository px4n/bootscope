package testutil

import (
	"bytes"
	"io"
	"os"
)

func CaptureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run function in a goroutine to prevent deadlock
	done := make(chan bool)
	var buf bytes.Buffer

	go func() {
		_, _ = io.Copy(&buf, r)
		done <- true
	}()

	f()

	w.Close()
	os.Stdout = old
	<-done

	return buf.String()
}

func CaptureStderr(f func()) string {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Run function in a goroutine to prevent deadlock
	done := make(chan bool)
	var buf bytes.Buffer

	go func() {
		_, _ = io.Copy(&buf, r)
		done <- true
	}()

	f()

	w.Close()
	os.Stderr = old
	<-done

	return buf.String()
}
