package test_utils

import (
	"runtime"
	"testing"
)

// Log stack trace and FailNow
func FailNowStackf(t *testing.T, msg string, msgArgs ...interface{}) {
	t.Logf(msg, msgArgs...)
	var stack [4096]byte
	runtime.Stack(stack[:], false)
	t.Logf("%s\n", stack[:])
	t.FailNow()
}

// if err isn't nil, FailNowStack()
func CheckError(err error, t *testing.T) {
	if err != nil {
		FailNowStackf(t, "%q\n", err)
	}
}
