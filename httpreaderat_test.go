package httpreaderat

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type readerAtFixture struct {
	suite.Suite
	server *httptest.Server
}

func (ra *readerAtFixture) AfterTest(suiteName, testName string) {
	if ra.server != nil {
		ra.server.Close()
	}
}

func TestReaderAtFixture(t *testing.T) {
	suite.Run(t, new(readerAtFixture))
}

func (ra *readerAtFixture) TestRangeNotSupported() {
	ra.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ra.Equal(r.Method, "GET")
		ra.Equal(r.URL.String(), "/file.zip")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"content":{"data": [1,2,3]}}`))
	}))

	req, err := http.NewRequest("GET", ra.server.URL+"/file.zip", nil)
	ra.Nil(err)
	reader, err := New(nil, req, nil)
	ra.EqualError(err, "server does not support range requests")
	ra.Nil(reader)
}

func (ra *readerAtFixture) TestNonGetRequestMethod() {
	req, err := http.NewRequest("POST", "http://not-valid.url/file.zip", nil)
	ra.Nil(err)
	reader, err := New(nil, req, nil)
	ra.EqualError(err, "invalid HTTP method")
	ra.Nil(reader)
}

func (ra *readerAtFixture) TestRangeSupportIntial() {
	ra.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rnge := r.Header.Get("Range")
		ra.Equal(rnge, "bytes=0-0")
		w.Header().Set("Content-Range", fmt.Sprintf(" bytes 0-0/%d", 1))
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte{17})
	}))

	req, err := http.NewRequest("GET", ra.server.URL+"/file.zip", nil)
	ra.Nil(err)
	reader, err := New(nil, req, nil)
	ra.Nil(err)
	ra.NotNil(reader)
}

func (ra *readerAtFixture) TestRangeSupportIntialEmptyResponse() {
	ra.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rnge := r.Header.Get("Range")
		ra.Equal(rnge, "bytes=0-0")
		w.Header().Set("Content-Range", fmt.Sprintf(" bytes 0-0/%d", 1))
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte{})
	}))

	req, err := http.NewRequest("GET", ra.server.URL+"/file.zip", nil)
	ra.Nil(err)
	reader, err := New(nil, req, nil)
	ra.EqualError(err, "content-length mismatch in http response")
	ra.Nil(reader)
}

func (ra *readerAtFixture) TestRangeSupportIntialTooMuchResponse() {
	ra.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rnge := r.Header.Get("Range")
		ra.Equal(rnge, "bytes=0-0")
		w.Header().Set("Content-Range", fmt.Sprintf(" bytes 0-0/%d", 1))
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte{17, 18, 19}) // should be 1
	}))

	req, err := http.NewRequest("GET", ra.server.URL+"/file.zip", nil)
	ra.Nil(err)
	reader, err := New(nil, req, nil)
	ra.EqualError(err, "content-length mismatch in http response")
	ra.Nil(reader)
}
