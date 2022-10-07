package utf8bom_test

import (
	"bytes"
	"testing"

	"github.com/bxcodec/faker/v3"
	"github.com/stretchr/testify/assert"

	"github.com/88labs/go-utils/utf8bom"
)

func TestAddBOM(t *testing.T) {
	fixtures := bytes.NewBufferString(faker.Name())
	withBom := utf8bom.AddBOM(fixtures.Bytes())
	assert.Equal(t, len(fixtures.Bytes())+len(utf8bom.BOM), len(withBom))
	assert.Equal(t, utf8bom.BOM, withBom[:len(utf8bom.BOM)])
}

func TestRemoveBOM(t *testing.T) {
	t.Run("with bom", func(t *testing.T) {
		fixtures := bytes.NewBufferString(faker.Name())
		withBom := utf8bom.AddBOM(fixtures.Bytes())

		removeBom := utf8bom.RemoveBOM(withBom)
		assert.Equal(t, fixtures.Bytes(), removeBom)
	})
	t.Run("not with bom", func(t *testing.T) {
		fixtures := bytes.NewBufferString(faker.Name())
		removeBom := utf8bom.RemoveBOM(fixtures.Bytes())
		assert.Equal(t, fixtures.Bytes(), removeBom)
	})
	t.Run("1 byte", func(t *testing.T) {
		fixtures := bytes.NewBufferString("a")
		removeBom := utf8bom.RemoveBOM(fixtures.Bytes())
		assert.Equal(t, fixtures.Bytes(), removeBom)
	})
}
