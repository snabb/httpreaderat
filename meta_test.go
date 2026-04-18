package httpreaderat

import (
	"net/http"
	"testing"
)

func TestValidateIgnoresContentTypeChanges(t *testing.T) {
	ra := &HTTPReaderAt{
		meta: meta{
			size:        5,
			contentType: "text/plain",
		},
	}

	resp := &http.Response{
		StatusCode: http.StatusPartialContent,
		Header: http.Header{
			"Content-Range": []string{"bytes 0-0/5"},
			"Content-Type":  []string{"application/octet-stream"},
		},
	}

	if err := ra.validate(resp); err != nil {
		t.Fatalf("validate() error = %v, want nil", err)
	}
}

func TestGetMetaFromStatusOK(t *testing.T) {
	resp := &http.Response{
		StatusCode:    http.StatusOK,
		ContentLength: 42,
		Header: http.Header{
			"Content-Type":  []string{"application/zip"},
			"Last-Modified": []string{"Wed, 21 Oct 2015 07:28:00 GMT"},
			"Etag":          []string{`"v1"`},
		},
	}

	got := getMeta(resp)
	if got.size != 42 {
		t.Fatalf("size = %d, want 42", got.size)
	}
	if got.contentType != "application/zip" {
		t.Fatalf("contentType = %q, want %q", got.contentType, "application/zip")
	}
	if got.lastModified != "Wed, 21 Oct 2015 07:28:00 GMT" {
		t.Fatalf("lastModified = %q", got.lastModified)
	}
	if got.etag != `"v1"` {
		t.Fatalf("etag = %q, want %q", got.etag, `"v1"`)
	}
}
