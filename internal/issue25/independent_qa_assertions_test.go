package issue25

import "testing"

// qaRequire and qaRequiref keep fatal regression assertions intact.
func qaRequire(t *testing.T, ok bool, args ...any) {
 t.Helper()
 if !ok { t.Fatal(args...) }
}
func qaRequiref(t *testing.T, ok bool, format string, args ...any) {
 t.Helper()
 if !ok { t.Fatalf(format, args...) }
}
