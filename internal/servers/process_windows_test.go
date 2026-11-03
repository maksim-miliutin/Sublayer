//go:build windows && (amd64 || arm64)

package servers

import (
	"testing"
	"unsafe"
)

func TestJobLimitsKeepTheLayoutWindowsExpects(t *testing.T) {
	if got := unsafe.Sizeof(extendedLimits{}); got != 144 {
		t.Errorf("JOBOBJECT_EXTENDED_LIMIT_INFORMATION is %d bytes, want 144", got)
	}
}
