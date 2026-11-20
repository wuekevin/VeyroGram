package imgw_test

import (
	"testing"

	"veyrogram/api/internal/platform/testim"
)

func startIM(t *testing.T, hook string) *testim.IM { return testim.Start(t, hook) }
