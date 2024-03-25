package hashutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/88labs/go-utils/hashutil"
	"github.com/go-faker/faker/v4"
)

func TestMustGetHash(t *testing.T) {
	t.Run("get hash", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			data := faker.Paragraph()
			h := hashutil.MustString(data)
			assert.Equal(t, 44, len(h))
		}
	})
}

func TestGetHash(t *testing.T) {
	t.Run("get hash", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			data := faker.Paragraph()
			h, err := hashutil.String(data)
			assert.NoError(t, err)
			assert.Equal(t, 44, len(h))
		}
	})
}

func TestMustGetHashByte(t *testing.T) {
	t.Run("get hash byte", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			data := []byte(faker.Paragraph())
			h := hashutil.MustByte(data)
			assert.Equal(t, 44, len(h))
		}
	})
}

func TestGetHashByte(t *testing.T) {
	t.Run("get hash byte", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			data := []byte(faker.Paragraph())
			h, err := hashutil.Byte(data)
			assert.NoError(t, err)
			assert.Equal(t, 44, len(h))
		}
	})
}

func TestMustGetHashStruct(t *testing.T) {
	type Test struct {
		ID   string
		hoge string
	}
	t.Run("get hash", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			data1 := Test{
				ID:   faker.Paragraph(),
				hoge: faker.Paragraph(),
			}
			data2 := Test{
				ID:   faker.Paragraph(),
				hoge: faker.Paragraph(),
			}
			h1 := hashutil.MustStruct(data1)
			assert.Equal(t, 44, len(h1))
			h2 := hashutil.MustStruct(data1)
			assert.Equal(t, h1, h2)
			h3 := hashutil.MustStruct(data2)
			assert.NotEqual(t, h1, h3)
		}
	})
	t.Run("get hash pointer", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			data1 := &Test{
				ID:   faker.Paragraph(),
				hoge: faker.Paragraph(),
			}
			data2 := &Test{
				ID:   faker.Paragraph(),
				hoge: faker.Paragraph(),
			}
			h1 := hashutil.MustStruct(data1)
			assert.Equal(t, 44, len(h1))
			h2 := hashutil.MustStruct(data1)
			assert.Equal(t, h1, h2)
			h3 := hashutil.MustStruct(data2)
			assert.NotEqual(t, h1, h3)
		}
	})
}

func TestGetHashStruct(t *testing.T) {
	type Test struct {
		ID   string
		hoge string
	}
	t.Run("get hash", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			data1 := Test{
				ID:   faker.Paragraph(),
				hoge: faker.Paragraph(),
			}
			data2 := Test{
				ID:   faker.Paragraph(),
				hoge: faker.Paragraph(),
			}
			h1, err := hashutil.Struct(data1)
			assert.NoError(t, err)
			assert.Equal(t, 44, len(h1))
			h2, err := hashutil.Struct(data1)
			assert.NoError(t, err)
			assert.Equal(t, h1, h2)
			h3, err := hashutil.Struct(data2)
			assert.NoError(t, err)
			assert.NotEqual(t, h1, h3)
		}
	})
	t.Run("get hash pointer", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			data1 := &Test{
				ID:   faker.Paragraph(),
				hoge: faker.Paragraph(),
			}
			data2 := &Test{
				ID:   faker.Paragraph(),
				hoge: faker.Paragraph(),
			}
			h1, err := hashutil.Struct(data1)
			assert.NoError(t, err)
			assert.Equal(t, 44, len(h1))
			h2, err := hashutil.Struct(data1)
			assert.NoError(t, err)
			assert.Equal(t, h1, h2)
			h3, err := hashutil.Struct(data2)
			assert.NoError(t, err)
			assert.NotEqual(t, h1, h3)
		}
	})
}
