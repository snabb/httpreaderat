package httprdrat

import (
	"bytes"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"os"
)

// Should we call this "ReadFromReadAtCloser"? :)
type Store interface {
	io.ReaderFrom
	io.ReaderAt
	io.Closer
}

// NewDefaultStore creates a Store with default settings. It
// buffers 1 MB in memory and if that is exceeded, up to 1 GB to file.
// Returned Store must be Close()d if it is no longer used.
func NewDefaultStore() Store {
	return NewLimitedStore(
		NewStoreMemory(), 1024*1024, NewLimitedStore(
			NewStoreFile(), 1024*1024*1024, nil))
}

type StoreFile struct {
	tmpfile *os.File
	size    int64
}

var _ Store = (*StoreFile)(nil)

func NewStoreFile() *StoreFile {
	return &StoreFile{}
}

func (s *StoreFile) ReadFrom(r io.Reader) (n int64, err error) {
	if s.tmpfile != nil {
		s.Close()
	}
	s.tmpfile, err = ioutil.TempFile("", "gotmp")
	if err != nil {
		return 0, err
	}
	n, err = io.Copy(s.tmpfile, r)
	s.size = n
	return n, err
}

func (s *StoreFile) ReadAt(p []byte, off int64) (n int, err error) {
	return s.tmpfile.ReadAt(p, off)
}

func (s *StoreFile) Size() int64 {
	return s.size
}

// Close must be called when the StoreFile is not used any more. It
// deletes the temporary file.
func (s *StoreFile) Close() error {
	if s.tmpfile == nil {
		return nil
	}
	name := s.tmpfile.Name()
	err := s.tmpfile.Close()
	err2 := os.Remove(name)
	s.tmpfile = nil
	s.size = 0

	if err == nil && err2 != nil {
		err = err2
	}
	return err
}

type StoreMemory struct {
	buf bytes.Buffer
}

var _ Store = (*StoreMemory)(nil)

func NewStoreMemory() *StoreMemory {
	return &StoreMemory{}
}

func (s *StoreMemory) ReadFrom(r io.Reader) (n int64, err error) {
	s.buf.Reset()
	return s.buf.ReadFrom(r)
}

func (s *StoreMemory) ReadAt(p []byte, off int64) (n int, err error) {
	rdr := bytes.NewReader(s.buf.Bytes())
	return rdr.ReadAt(p, off)
}

func (s *StoreMemory) Size() int64 {
	return int64(s.buf.Len())
}

// Close may be called but it is not necessary.
func (s *StoreMemory) Close() error {
	s.buf.Reset()
	return nil
}

type LimitedStore struct {
	s        Store
	limit    int64
	fallback Store
	fellback bool
}

var _ Store = (*LimitedStore)(nil)

var ErrStoreLimit = errors.New("backing store limit reached")

func NewLimitedStore(s Store, limit int64, fallback Store) *LimitedStore {
	return &LimitedStore{
		s:        s,
		limit:    limit,
		fallback: fallback,
	}
}

func (s *LimitedStore) ReadFrom(r io.Reader) (n int64, err error) {
	// XXX
	if s.fellback == true {
		return s.s.ReadFrom(r)
	}

	lr := io.LimitReader(r, s.limit)

	n, err = s.s.ReadFrom(lr)
	if n < s.limit {
		s.limit -= n
		return n, err
	}

	if s.fallback == nil {
		return n, ErrStoreLimit
	}

	srdr := io.NewSectionReader(s.s, 0, n)
	n, err = s.fallback.ReadFrom(io.MultiReader(srdr, r))

	s.s.Close()
	s.s = s.fallback
	s.fellback = true

	return n, err
}

func (s *LimitedStore) ReadAt(p []byte, off int64) (n int, err error) {
	return s.s.ReadAt(p, off)
}

func (s *LimitedStore) Close() error {
	return s.s.Close()
}
