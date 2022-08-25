package ulid_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

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
