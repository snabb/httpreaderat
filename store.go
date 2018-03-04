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

func (bs *StoreFile) ReadFrom(r io.Reader) (n int64, err error) {
	if bs.tmpfile == nil {
		bs.tmpfile, err = ioutil.TempFile("", "tmpbs")
		if err != nil {
			return 0, err
		}
	}
	n, err = io.Copy(bs.tmpfile, r)
	bs.size += n
	return n, err
}

func (bs *StoreFile) ReadAt(p []byte, off int64) (n int, err error) {
	return bs.tmpfile.ReadAt(p, off)
}

func (bs *StoreFile) Size() int64 {
	return bs.size
}

// Close must be called when the StoreFile is not used any more. It
// deletes the temporary file.
func (bs *StoreFile) Close() error {
	if bs.tmpfile == nil {
		return nil
	}
	name := bs.tmpfile.Name()
	err := bs.tmpfile.Close()
	err2 := os.Remove(name)
	bs.tmpfile = nil
	bs.size = 0

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

func (bs *StoreMemory) ReadFrom(r io.Reader) (n int64, err error) {
	return bs.buf.ReadFrom(r)
}

func (bs *StoreMemory) ReadAt(p []byte, off int64) (n int, err error) {
	rdr := bytes.NewReader(bs.buf.Bytes())
	return rdr.ReadAt(p, off)
}

func (bs *StoreMemory) Size() int64 {
	return int64(bs.buf.Len())
}

// Close may be called but it is not necessary.
func (bs *StoreMemory) Close() error {
	bs.buf.Reset()
	return nil
}

type LimitedStore struct {
	bs       Store
	limit    int64
	fallback Store
	fellback bool
}

var _ Store = (*LimitedStore)(nil)

var ErrStoreLimit = errors.New("backing store limit reached")

func NewLimitedStore(bs Store, limit int64, fallback Store) *LimitedStore {
	return &LimitedStore{
		bs:       bs,
		limit:    limit,
		fallback: fallback,
	}
}

func (bs *LimitedStore) ReadFrom(r io.Reader) (n int64, err error) {
	if bs.fellback == true {
		return bs.bs.ReadFrom(r)
	}

	lr := io.LimitReader(r, bs.limit)

	n, err = bs.bs.ReadFrom(lr)
	if n < bs.limit {
		bs.limit -= n
		return n, err
	}

	if bs.fallback == nil {
		return n, ErrStoreLimit
	}

	bsrdr := io.NewSectionReader(bs.bs, 0, n)
	n, err = bs.fallback.ReadFrom(io.MultiReader(bsrdr, r))

	bs.bs.Close()

	bs.bs = bs.fallback
	bs.fellback = true

	return n, err
}

func (bs *LimitedStore) ReadAt(p []byte, off int64) (n int, err error) {
	return bs.bs.ReadAt(p, off)
}

func (bs *LimitedStore) Close() error {
	return bs.bs.Close()
}
