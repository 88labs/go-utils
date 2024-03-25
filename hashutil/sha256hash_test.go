package hashutil_test

import (
	"testing"

	"github.com/88labs/go-utils/hashutil"
	"github.com/go-faker/faker/v4"
)

func TestMustGetHash(t *testing.T) {
	t.Run("get hash", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			data := faker.Paragraph()
			h := hashutil.MustGetHash(data)
			if 44 != len(h) {
				t.Errorf("Expected h to not equal 44")
			}
		}
	})
}

func TestGetHash(t *testing.T) {
	t.Run("get hash", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			data := faker.Paragraph()
			h, err := hashutil.GetHash(data)
			if err != nil {
				t.Error(err)
			}
			if 44 != len(h) {
				t.Errorf("Expected h to not equal 44")
			}
		}
	})
}

func TestMustGetHashByte(t *testing.T) {
	t.Run("get hash byte", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			data := []byte(faker.Paragraph())
			h := hashutil.MustGetHashByte(data)
			if 44 != len(h) {
				t.Errorf("Expected h to not equal 44")
			}
		}
	})
}

func TestGetHashByte(t *testing.T) {
	t.Run("get hash byte", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			data := []byte(faker.Paragraph())
			h, err := hashutil.GetHashByte(data)
			if err != nil {
				t.Error(err)
			}
			if 44 != len(h) {
				t.Errorf("Expected h to not equal 44")
			}
		}
	})
}
