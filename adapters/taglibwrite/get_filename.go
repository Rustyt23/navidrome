//go:build !windows

package taglibwrite

import "C"

func getFilename(s string) *C.char {
	return C.CString(s)
}
