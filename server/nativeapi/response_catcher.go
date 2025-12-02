package nativeapi

import (
	"bytes"
	"net/http"
)

type responseCatcher struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	buf         bytes.Buffer
}

func newResponseCatcher(w http.ResponseWriter) *responseCatcher {
	return &responseCatcher{ResponseWriter: w, status: http.StatusOK}
}

func (r *responseCatcher) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.status = status
	r.wroteHeader = true
}

func (r *responseCatcher) Write(b []byte) (int, error) {
	return r.buf.Write(b)
}

func (r *responseCatcher) Flush() {
	if !r.wroteHeader {
		r.status = http.StatusOK
	}
	r.ResponseWriter.WriteHeader(r.status)
	if r.buf.Len() > 0 {
		_, _ = r.ResponseWriter.Write(r.buf.Bytes())
	}
}
