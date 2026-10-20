package issue25

import "testing"

// qaReject and qaRejectf preserve fatal assertions without inflating test complexity.
func qaReject(t *testing.T, invalid bool, args ...any) {
 t.Helper()
 if invalid { t.Fatal(args...) }
}
func qaRejectf(t *testing.T, invalid bool, format string, args ...any) {
 t.Helper()
 if invalid { t.Fatalf(format, args...) }
}
