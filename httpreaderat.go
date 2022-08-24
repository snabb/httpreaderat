// Package httpreaderat implements io.ReaderAt that makes HTTP Range Requests.
//
// It can be used for example with "archive/zip" package in Go standard
// library. Together they can be used to access remote (HTTP accessible)
// ZIP archives without needing to download the whole archive file.
//
// HTTP Range Requests (see RFC 7233) are used to retrieve the requested
// byte range.
package httpreaderat

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/snabb/httpreaderat/pkg/contentrange"
)

// HTTPReaderAt is io.ReaderAt implementation that makes HTTP Range Requests.
// New instances must be created with the New() function.
// It is safe for concurrent use.
type HTTPReaderAt struct {
	client *http.Client
	req    *http.Request
	meta   meta

	bs    Store
	usebs bool
}

var _ io.ReaderAt = (*HTTPReaderAt)(nil)

// ErrValidationFailed error is returned if the file changed under
// our feet.
var ErrValidationFailed = errors.New("validation failed")

// ErrNoRange error is returned if the server does not support range
// requests and there is no Store defined for buffering the file.
var ErrNoRange = errors.New("server does not support range requests")

// New creates a new HTTPReaderAt. If nil is passed as http.Client, then
// http.DefaultClient is used. The supplied http.Request is used as a
// prototype for requests. It is copied before making the actual request.
// It is an error to specify any other HTTP method than "GET".
// A Store can be supplied to enable fallback mechanism in case
// the server does not support HTTP Range Requests.
func New(client *http.Client, req *http.Request, bs Store) (ra *HTTPReaderAt, err error) {
	if client == nil {
		client = http.DefaultClient
	}
	if req.Method != "GET" {
		return nil, errors.New("invalid HTTP method")
	}
	ra = &HTTPReaderAt{
		client: client,
		req:    req,
		bs:     bs,
	}
	// Make 1 byte Range Request to see if they are supported or not.
	// Also stores the file metadata for later use.
	_, err = ra.readAt(make([]byte, 1), 0, true)
	if err != nil {
		return nil, err
	}
	return ra, nil
}

// ContentType returns "Content-Type" header contents.
func (ra *HTTPReaderAt) ContentType() string {
	return ra.meta.contentType
}

// LastModified returns "Last-Modified" header contents.
func (ra *HTTPReaderAt) LastModified() string {
	return ra.meta.lastModified
}

// Size returns the size of the file.
func (ra *HTTPReaderAt) Size() int64 {
	return ra.meta.size
}

// ReadAt reads len(b) bytes from the remote file starting at byte offset
// off. It returns the number of bytes read and the error, if any. ReadAt
// always returns a non-nil error when n < len(b). At end of file, that
// error is io.EOF. It is safe for concurrent use.
//
// It tries to notice if the file changes by tracking the size as well as
// Content-Type, Last-Modified and ETag headers between consecutive ReadAt
// calls. In case any change is detected, ErrValidationFailed is returned.
func (ra *HTTPReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	return ra.readAt(p, off, false)
}

func (ra *HTTPReaderAt) readAt(p []byte, off int64, initialize bool) (int, error) {
	if ra.usebs {
		return ra.bs.ReadAt(p, off)
	}
	// fmt.Printf("readat off=%d len=%d\n", off, len(p))
	if len(p) == 0 {
		return 0, nil
	}
	req := ra.copyReq()

	reqFirst := off
	reqLast := off + int64(len(p)) - 1

	var returnErr error
	if !initialize && ra.meta.size != -1 && reqLast > ra.meta.size-1 {
		// Clamp down the requested range because some servers return
		// "416 Range Not Satisfiable" if trying to read past the end of the file.
		reqLast = ra.meta.size - 1
		returnErr = io.EOF
		if reqLast < reqFirst {
			return 0, io.EOF
		}
		p = p[:reqLast-reqFirst+1]
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", reqFirst, reqLast))

	resp, err := ra.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return 0, fmt.Errorf("http request error: %s", resp.Status)
	}

	if initialize {
		ra.meta = getMeta(resp)
	} else {
		err = ra.validate(resp)
		if err != nil {
			return 0, err
		}
	}

	if resp.StatusCode == http.StatusOK {
		if ra.bs == nil {
			return 0, ErrNoRange
		}
		if !initialize {
			return 0, errors.New("server suddenly stopped supporting range requests")
		}
		// The following code path is not thread safe.
		// We end up here only from New
		// (initialize == true) and at that point concurrency
		// is not possible.

		ra.usebs = true
		size, err := ra.bs.ReadFrom(resp.Body)
		if resp.ContentLength != -1 && resp.ContentLength != size {
			// meta size does not match body size, should we care? XXX
		}
		if resp.ContentLength == -1 {
			ra.meta.size = size
		}

		if err != nil {
			return int(size), err
		}
		return ra.bs.ReadAt(p, off)
	}

	contentRange := resp.Header.Get("Content-Range")
	if contentRange == "" {
		return 0, errors.New("no content-range header in partial response")
	}
	first, last, _, err := contentrange.Parse(contentRange)
	if err != nil {
		return 0, fmt.Errorf("http request: %w", err)
	}
	if first != reqFirst || last > reqLast {
		return 0, fmt.Errorf(
			"received different range than requested (req=%d-%d, resp=%d-%d)",
			reqFirst, reqLast, first, last)
	}
	if resp.ContentLength != last-first+1 {
		return 0, errors.New("content-length mismatch in http response")
	}

	n, err := io.ReadFull(resp.Body, p)
	if err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			err = io.EOF
		}
		return 0, err
	}

	if (err == nil || err == io.EOF) && int64(n) != resp.ContentLength {
		// XXX body size was different from the ContentLength
		// header? should we do something about it? return error?
	}
	if err == nil && returnErr != nil {
		err = returnErr
	}
	return n, err
}

func cloneHeader(h http.Header) http.Header {
	h2 := make(http.Header, len(h))
	for k, vv := range h {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		h2[k] = vv2
	}
	return h2
}

func (ra *HTTPReaderAt) copyReq() *http.Request {
	out := *ra.req
	out.Body = nil
	out.ContentLength = 0
	out.Header = cloneHeader(ra.req.Header)

	return &out
}

func (ra *HTTPReaderAt) validate(resp *http.Response) (err error) {
	m := getMeta(resp)

	if ra.meta.size != m.size ||
		ra.meta.lastModified != m.lastModified ||
		ra.meta.etag != m.etag {
		return ErrValidationFailed
	}
	return nil
}

type meta struct {
	size         int64
	lastModified string
	etag         string
	contentType  string
}

func getMeta(resp *http.Response) (meta meta) {
	meta.lastModified = resp.Header.Get("Last-Modified")
	meta.etag = resp.Header.Get("ETag")
	meta.contentType = resp.Header.Get("Content-Type")

	switch resp.StatusCode {
	case http.StatusOK:
		meta.size = resp.ContentLength
	case http.StatusPartialContent:
		contentRange := resp.Header.Get("Content-Range")
		if contentRange != "" {
			_, _, meta.size, _ = contentrange.Parse(contentRange)
		}
	}
	return meta
}
