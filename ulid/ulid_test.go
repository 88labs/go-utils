package ulid_test

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	oklogulid "github.com/oklog/ulid/v2"

	"github.com/88labs/go-utils/ulid"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		err   string
	}{
		{"zero", "00000000000000000000000000", "ulid is zero"},
		{"ok", "0000XSNJG0MQJHBF4QX1EFD6Y3", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := ulid.Parse(tt.input)
			if len(tt.err) == 0 {
				if err != nil {
					t.Fatalf("want no err, but has err %v", err)
				}
				if id.IsZero() {
					t.Fatal("ulid is zero")
				}
			}

			if len(tt.err) > 0 {
				if err == nil {
					t.Fatalf("want %v, but %v", tt.err, err)
				}
				if !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("want %v, but %v", tt.err, err)
				}
			}
		})
	}
}

func TestParseStrict(t *testing.T) {
	tests := []struct {
		name  string
		input string
		err   string
	}{
		{"zero", "00000000000000000000000000", "ulid is zero"},
		{"ok", "0000XSNJG0MQJHBF4QX1EFD6Y3", ""},
	}
	base := "0000XSNJG0MQJHBF4QX1EFD6Y3"
	for i := 0; i < oklogulid.EncodedSize; i++ {
		tests = append(tests, struct {
			name  string
			input string
			err   string
		}{
			name:  fmt.Sprintf("Invalid 0xFF at index %d", i),
			input: base[:i] + "\xff" + base[i+1:],
			err:   oklogulid.ErrInvalidCharacters.Error(),
		})
		tests = append(tests, struct {
			name  string
			input string
			err   string
		}{
			name:  fmt.Sprintf("Invalid 0x00 at index %d", i),
			input: base[:i] + "\x00" + base[i+1:],
			err:   oklogulid.ErrInvalidCharacters.Error(),
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := ulid.Parse(tt.input)
			if len(tt.err) == 0 {
				if err != nil {
					t.Fatalf("want no err, but has err %v", err)
				}
				if id.IsZero() {
					t.Fatal("ulid is zero")
				}
			}

			if len(tt.err) > 0 {
				if err == nil {
					t.Fatalf("want %v, but %v", tt.err, err)
				}
				if !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("want %v, but %v", tt.err, err)
				}
			}
		})
	}
}

func TestMustNew(t *testing.T) {
	t.Run("ulidnew", func(t *testing.T) {
		var wg sync.WaitGroup
		num := 100000
		wg.Add(num)
		for i := 0; i < num; i++ {
			go func() {
				defer wg.Done()
				id := ulid.MustNew()
				if id.IsZero() {
					panic(fmt.Sprintf("zero: %v", id))
				}
			}()
		}
		wg.Wait()
	})
}

func TestSetTime(t *testing.T) {
	t.Run("set time", func(t *testing.T) {
		id := ulid.MustNew()
		if id.Time().IsZero() {
			t.Fatal("zero")
		}
		now := time.Now().UTC()
		err := id.SetTime(now)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("set time 50 year after", func(t *testing.T) {
		id := ulid.MustNew()
		if id.Time().IsZero() {
			t.Fatal("zero")
		}
		after := time.Now().AddDate(50, 0, 0).UTC()
		err := id.SetTime(after)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("set time zero", func(t *testing.T) {
		id := ulid.MustNew()
		if id.Time().IsZero() {
			t.Fatal("zero")
		}
		zero := time.Date(0, 0, 0, 0, 0, 0, 0, time.UTC)
		err := id.SetTime(zero)
		if !errors.Is(err, ulid.ErrInvalidTime) {
			t.Fatal("err not match")
		}
	})
	t.Run("set time too big", func(t *testing.T) {
		id := ulid.MustNew()
		if id.Time().IsZero() {
			t.Fatal("zero")
		}
		big := time.Date(99999, 12, 31, 23, 59, 59, 0, time.UTC)
		err := id.SetTime(big)
		if !errors.Is(err, ulid.ErrInvalidTime) {
			t.Fatal("err not match")
		}
	})
}

func TestTime(t *testing.T) {
	t.Run("get time", func(t *testing.T) {
		id := ulid.MustNew()
		if id.Time().IsZero() {
			t.Fatal("zero")
		}
		now := time.Now().UTC()
		if err := id.SetTime(now); err != nil {
			t.Fatal(err)
		}
		got := id.Time()
		cmp := now.Sub(got)
		if cmp > time.Millisecond {
			t.Fatal("time not match", cmp)
		}
	})
}
