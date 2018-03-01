// Package htreaderat implements io.ReaderAt for http URLs.
package htreaderat

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type ReaderAt struct {
	htc *http.Client
	ctx context.Context
	url string

	metaSet bool
	meta
}

var _ io.ReaderAt = (*ReaderAt)(nil)

func New(htc *http.Client, url string, ctx context.Context) (ra *ReaderAt) {
	if htc == nil {
		htc = http.DefaultClient
	}

	return &ReaderAt{
		htc: htc,
		ctx: ctx,
		url: url,
	}
}

func etagStrongMatch(a, b string) bool {
	return a == b && a != "" && a[0] == '"'
}

func (ra *ReaderAt) setAndValidate(resp *http.Response) (ok bool) {
	m := getMeta(resp)

	if ra.metaSet == false {
		ra.meta = m
		ra.metaSet = true
		return true
	}

	return ra.size == m.size &&
		ra.lastModified == m.lastModified &&
		etagStrongMatch(ra.etag, m.etag)
}

var ErrValidationFailed = errors.New("file validation error")

func (ra *ReaderAt) setMeta() error {
	if ra.metaSet == false {
		req, err := http.NewRequest("HEAD", ra.url, nil)
		if err != nil {
			return errors.Wrap(err, "error forming http request")
		}
		if ra.ctx != nil {
			req = req.WithContext(ra.ctx)
		}
		resp, err := ra.htc.Do(req)
		if err != nil {
			return errors.Wrap(err, "http request error")
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return errors.Errorf("http request error: %s", resp.Status)
		}
		ok := ra.setAndValidate(resp)
		if !ok {
			return ErrValidationFailed
		}
	}
	return nil
}

func (ra *ReaderAt) Size() (int64, error) {
	err := ra.setMeta()
	return ra.size, err
}

func (ra *ReaderAt) ContentType() (string, error) {
	err := ra.setMeta()
	return ra.contentType, err
}

func (ra *ReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	// fmt.Printf("readat off=%d len=%d\n", off, len(p))
	if len(p) == 0 {
		return 0, nil
	}
	req, err := http.NewRequest("GET", ra.url, nil)
	if err != nil {
		return 0, errors.Wrap(err, "error forming http request")
	}
	if ra.ctx != nil {
		req = req.WithContext(ra.ctx)
	}
	reqFirst := off
	reqLast := off + int64(len(p)) - 1
	reqRange := fmt.Sprintf("bytes=%d-%d", reqFirst, reqLast)
	req.Header.Set("Range", reqRange)

	resp, err := ra.htc.Do(req)
	if err != nil {
		return 0, errors.Wrap(err, "http request error")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return 0, errors.Errorf("http request error: %s", resp.Status)
	}
	if !ra.setAndValidate(resp) {
		return 0, ErrValidationFailed
	}
	if resp.StatusCode == http.StatusOK {
		return 0, errors.New("received full response, fallback not implemented yet")
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
		return 0, errors.New("wrong content-length in http response")
	}
	n, err = io.ReadFull(resp.Body, p)
	return n, err
}

var errParse = errors.New("content-range parse error")

// Content-Range: bytes 42-1233/1234
// Content-Range: bytes 42-1233/*
// Content-Range: bytes */1234
func parseContentRange(str string) (first, last, length int64, err error) {
	first, last, length = -1, -1, -1

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
