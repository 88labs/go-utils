package ulid

import (
	"crypto/rand"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
	"time"

	oklogulid "github.com/oklog/ulid/v2"
)

type ULID oklogulid.ULID

var (
	pool = sync.Pool{
		New: func() interface{} { return oklogulid.Monotonic(rand.Reader, 0) },
	}
	zeroValueULID  oklogulid.ULID
	ErrULIDZero    = errors.New("ulid is zero")
	ErrInvalidTime = errors.New("invalid time")
)

func New() (ULID, error) {
	var entropy = rand.Reader
	if e, ok := pool.Get().(io.Reader); ok {
		entropy = e
		defer pool.Put(e)
	}
	id, err := oklogulid.New(oklogulid.Now(), entropy)
	if err != nil {
		return ULID{}, err
	}
	return ULID(id), nil
}

func MustNew() ULID {
	id, err := New()
	if err != nil {
		panic(err)
	}
	return id
}

func MustParse(s string) ULID {
	u := ULID(oklogulid.MustParseStrict(s))
	if u.IsZero() {
		panic(ErrULIDZero)
	}
	return u
}

func Parse(s string) (ULID, error) {
	oklogULID, err := oklogulid.ParseStrict(s)
	u := ULID(oklogULID)
	if err != nil {
		return u, err
	}
	if u.IsZero() {
		return u, ErrULIDZero
	}

	return u, nil
}

// ParseStrict parses an encoded ULID, returning an error in case of failure.
//
// It is like Parse, but additionally validates that the parsed ULID consists
// only of valid base32 characters. It is slightly slower than Parse.
func ParseStrict(id string) (ULID, error) {
	okid, err := oklogulid.ParseStrict(id)
	if err != nil {
		return ULID{}, err
	}
	return ULID(okid), nil
}

func (u ULID) Bytes() []byte {
	return oklogulid.ULID(u).Bytes()
}

func (u ULID) String() string {
	return oklogulid.ULID(u).String()
}

func (u ULID) IsZero() bool {
	return oklogulid.ULID(u) == zeroValueULID
}

func (u ULID) SetTime(t time.Time) error {
	id := oklogulid.ULID(u)
	err := id.SetTime(oklogulid.Timestamp(t))
	if err != nil {
		return errors.Join(ErrInvalidTime, err)
	}
	return nil
}

func (u ULID) Time() time.Time {
	return oklogulid.Time(oklogulid.ULID(u).Time())
}

func (u ULID) Scan(src any) error {
	_u := oklogulid.ULID(u)
	return _u.Scan(src)
}

func (u ULID) Value() (driver.Value, error) {
	_u := oklogulid.ULID(u)
	return _u.Value()
}
