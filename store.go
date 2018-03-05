package httpreaderat

import (
	"bytes"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"os"
)

// Store is the interface to a temporary byte storage. Calling ReadFrom
// with io.Reader reads data to a temporary storage and allows it to be
// read back with ReadAt. A Store must be Closed to free up the space when
// it is no longer needed. A Store can be reused by filling it with new
// data. ReadFrom is not safe to be called concurrently. ReadAt is safe
// for concurrent use.
type Store interface {
	io.ReaderFrom
	io.ReaderAt
	io.Closer
}

// NewDefaultStore creates a Store with default settings. It buffers up to
// 1 MB in memory and if that is exceeded, up to 1 GB to a temporary file.
// Returned Store must be Closed if it is no longer needed.
func NewDefaultStore() Store {
	return NewLimitedStore(
		NewStoreMemory(), 1024*1024, NewLimitedStore(
			NewStoreFile(), 1024*1024*1024, nil))
}

// StoreFile takes data from io.Reader and provides io.ReaderAt backed by
// a temporary file. It implements the Store interface.
type StoreFile struct {
	tmpfile *os.File
	size    int64
}

var _ Store = (*StoreFile)(nil)

func NewStoreFile() *StoreFile {
	return &StoreFile{}
}

// Read and store the contents of r to a temporary file. Previous contents
// (if any) are erased. Can not be called concurrently.
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

// ReadAt reads len(b) bytes from the Store starting at byte offset off. It
// returns the number of bytes read and the error, if any. ReadAt always
// returns a non-nil error when n < len(b). At end of file, that error is
// io.EOF. It is safe for concurrent use.
func (s *StoreFile) ReadAt(p []byte, off int64) (n int, err error) {
	if s.tmpfile == nil {
		return 0, nil
	}
	return s.tmpfile.ReadAt(p, off)
}

// Size returns the amount of data (in bytes) in the Store.
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

// StoreMemory takes data from io.Reader and provides io.ReaderAt backed by
// a memory buffer. It implements the Store interface.
type StoreMemory struct {
	rdr *bytes.Reader
}

var _ Store = (*StoreMemory)(nil)

func NewStoreMemory() *StoreMemory {
	return &StoreMemory{}
}

// Read and store the contents of r to a memory buffer. Previous contents
// (if any) are erased.
func (s *StoreMemory) ReadFrom(r io.Reader) (n int64, err error) {
	var buf bytes.Buffer
	n, err = buf.ReadFrom(r)
	s.rdr = bytes.NewReader(buf.Bytes())

	return n, err
}

// ReadAt reads len(b) bytes from the Store starting at byte offset off. It
// returns the number of bytes read and the error, if any. ReadAt always
// returns a non-nil error when n < len(b). At end of file, that error is
// io.EOF. It is safe for concurrent use.
func (s *StoreMemory) ReadAt(p []byte, off int64) (n int, err error) {
	if s.rdr == nil {
		return 0, nil
	}
	return s.rdr.ReadAt(p, off)
}

// Size returns the amount of data (in bytes) in the Store.
func (s *StoreMemory) Size() int64 {
	if s.rdr == nil {
		return 0
	}
	return s.rdr.Size()
}

// Close releases the memory buffer to be garbage collected.
func (s *StoreMemory) Close() error {
	s.rdr = nil
	return nil
}

// LimitedStore stores to a primary store up to a size limit. If the
// size limit is exceeded, a secondary store is used. If secondary store
// is nil, error is returned if the size limit is exceeded.
type LimitedStore struct {
	s         Store
	primary   Store
	limit     int64
	secondary Store
}

var _ Store = (*LimitedStore)(nil)

// ErrStoreLimit error is returned when LimitedStore's limit is reached
// and there is no secondary fallback Store defined.
var ErrStoreLimit = errors.New("store size limit reached")

// NewLimitedStore creates a new LimitedStore with the specified settings.
func NewLimitedStore(primary Store, limit int64, secondary Store) *LimitedStore {
	return &LimitedStore{
		primary:   primary,
		limit:     limit,
		secondary: secondary,
	}
}

// Store the contents of r to the primary store. If the size limit is
// reached, fall back to the secondary store or return ErrStoreLimit
// if secondary store is nil.
func (s *LimitedStore) ReadFrom(r io.Reader) (n int64, err error) {
	if s.s != nil {
		s.s.Close()
	}
	s.s = s.primary

	lr := io.LimitReader(r, s.limit)

	n, err = s.primary.ReadFrom(lr)
	if n < s.limit {
		return n, err
	}

	if s.secondary == nil {
		return n, ErrStoreLimit
	}

	// move already received data from primary store to secondary store
	srdr := io.NewSectionReader(s.primary, 0, n)
	n, err = s.secondary.ReadFrom(io.MultiReader(srdr, r))
	s.primary.Close()
	s.s = s.secondary

	return n, err
}

func (s *LimitedStore) ReadAt(p []byte, off int64) (n int, err error) {
	if s.s == nil {
		return 0, nil
	}
	return s.s.ReadAt(p, off)
}

func (s *LimitedStore) Close() error {
	if s.s == nil {
		return nil
	}
	err := s.s.Close()
	s.s = nil
	return err
}
