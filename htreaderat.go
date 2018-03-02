// Package htreaderat implements io.ReaderAt for making HTTP Range Requests.
//
// It can be used for example with "archive/zip" package in Go standard
// library. Together they can be used to access remote (HTTP accessible)
// ZIP archives without needing to download the whole archive file.
//
// HTTP Range Requests (see RFC 7233) are used to retrieve the requested
// byte range. Currently an error is returned if a remote server does not
// support Range Requests.
package htreaderat

import (
	"fmt"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// ReaderAt is io.ReaderAt implementation. New instances must be created
// with the New() function.
type ReaderAt struct {
	client *http.Client
	req    *http.Request

	metaMutex sync.Mutex
	metaSet   bool
	meta      meta
}

var _ io.ReaderAt = (*ReaderAt)(nil)

// ErrValidationFailed error is returned if the file changed under
// our feet.
var ErrValidationFailed = errors.New("validation failed")

// New creates a new ReaderAt. If nil is passed as http.Client, then
// http.DefaultClient is used. The supplied http.Request is used as a
// prototype for requests. It is copied before making the actual request.
// Only "GET" HTTP method is allowed.
func New(client *http.Client, req *http.Request) (ra *ReaderAt, err error) {
	if client == nil {
		client = http.DefaultClient
	}
	if req.Method != "GET" {
		return nil, errors.New("only GET HTTP method allowed")
	}

	return &ReaderAt{
		client: client,
		req:    req,
	}, nil
}

// ContentType returns "Content-Type" header contents.
func (ra *ReaderAt) ContentType() (string, error) {
	err := ra.setMeta()
	return ra.meta.contentType, err
}

// LastModified returns "Last-Modified" header contents.
func (ra *ReaderAt) LastModified() (string, error) {
	err := ra.setMeta()
	return ra.meta.lastModified, err
}

// Size returns the size of the file.
func (ra *ReaderAt) Size() (int64, error) {
	err := ra.setMeta()
	return ra.meta.size, err
}

// ReadAt reads len(p) bytes into p starting at offset off in the
// remote file. It returns the number of bytes read (0 <= n <= len(p))
// and any error encountered.
//
// When ReadAt returns n < len(p), it returns a non-nil error explaining
// why more bytes were not returned.
func (ra *ReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	// fmt.Printf("readat off=%d len=%d\n", off, len(p))
	if len(p) == 0 {
		return 0, nil
	}
	req := ra.copyReq()

	reqFirst := off
	reqLast := off + int64(len(p)) - 1
	reqRange := fmt.Sprintf("bytes=%d-%d", reqFirst, reqLast)
	req.Header.Set("Range", reqRange)

	resp, err := ra.client.Do(req)
	if err != nil {
		return 0, errors.Wrap(err, "http request error")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return 0, errors.Errorf("http request error: %s", resp.Status)
	}
	err = ra.setAndValidate(resp)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode == http.StatusOK {
		return 0, errors.New("server does not support range requests (fallback not implemented yet)")
	}
	contentRange := resp.Header.Get("Content-Range")
	if contentRange == "" {
		return 0, errors.New("no content-range header in partial response")
	}
	first, last, _, err := parseContentRange(contentRange)
	if err != nil {
		return 0, errors.Wrap(err, "http request error")
	}
	if first != reqFirst || last > reqLast {
		return 0, errors.Errorf(
			"received different range than requested (req=%d-%d, resp=%d-%d)",
			reqFirst, reqLast, first, last)
	}
	if resp.ContentLength != last-first+1 {
		return 0, errors.New("content-length mismatch in http response")
	}
	n, err = io.ReadFull(resp.Body, p)

	if err == io.ErrUnexpectedEOF {
		err = io.EOF
	}
	return n, err
}

var errParse = errors.New("content-range parse error")

func parseContentRange(str string) (first, last, length int64, err error) {
	first, last, length = -1, -1, -1

	// Content-Range: bytes 42-1233/1234
	// Content-Range: bytes 42-1233/*
	// Content-Range: bytes */1234
	// (Maybe I should have used regexp here instead of Splitting... :)

	strs := strings.Split(str, " ")
	if len(strs) != 2 || strs[0] != "bytes" {
		return -1, -1, -1, errParse
	}
	strs = strings.Split(strs[1], "/")
	if len(strs) != 2 {
		return -1, -1, -1, errParse
	}
	if strs[1] != "*" {
		length, err = strconv.ParseInt(strs[1], 10, 64)
		if err != nil {
			return -1, -1, -1, errParse
		}
	}
	if strs[0] != "*" {
		strs = strings.Split(strs[0], "-")
		if len(strs) != 2 {
			return -1, -1, -1, errParse
		}
		first, err = strconv.ParseInt(strs[0], 10, 64)
		if err != nil {
			return -1, -1, -1, errParse
		}
		last, err = strconv.ParseInt(strs[1], 10, 64)
		if err != nil {
			return -1, -1, -1, errParse
		}
	}
	if first == -1 && last == -1 && length == -1 {
		return -1, -1, -1, errParse
	}
	return first, last, length, nil
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
			_, _, meta.size, _ = parseContentRange(contentRange)
		}
	}
	return meta
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

func (ra *ReaderAt) copyReq() *http.Request {
	out := *ra.req
	out.Body = nil
	out.ContentLength = 0
	out.Header = cloneHeader(ra.req.Header)

	return &out
}

func (ra *ReaderAt) setAndValidate(resp *http.Response) (err error) {
	m := getMeta(resp)

	ra.metaMutex.Lock()
	defer ra.metaMutex.Unlock()

	if ra.metaSet == false {
		ra.meta = m
		ra.metaSet = true
		return nil
	}
	if ra.meta.size == m.size &&
		ra.meta.lastModified == m.lastModified &&
		ra.meta.etag == m.etag {
		return nil
	}
	return ErrValidationFailed
}

func (ra *ReaderAt) setMeta() error {
	ra.metaMutex.Lock()

	if ra.metaSet {
		ra.metaMutex.Unlock()
		return nil
	}

	ra.metaMutex.Unlock()

	req := ra.copyReq()
	req.Method = "HEAD"

	resp, err := ra.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "http request error")
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("http request error: %s", resp.Status)
	}
	err = ra.setAndValidate(resp)
	if err != nil {
		return err
	}
	return nil
}
