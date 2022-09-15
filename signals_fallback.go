//go:build !unix

package main

import (
	"os"
)

var interruptSignals = []os.Signal{
	os.Interrupt,
}
