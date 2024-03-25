package hashutil

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

type Hash string

func (h Hash) Value() string {
	return string(h)
}

func MustGetHash(str string) Hash {
	hasher := sha256.New()
	if _, err := hasher.Write([]byte(str)); err != nil {
		panic(err)
	}
	sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
	return Hash(sha)
}

func GetHash(str string) (Hash, error) {
	hasher := sha256.New()
	if _, err := hasher.Write([]byte(str)); err != nil {
		return "", err
	}
	sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
	return Hash(sha), nil
}

func MustGetHashByte(bytes []byte) Hash {
	hasher := sha256.New()
	if _, err := hasher.Write(bytes); err != nil {
		panic(err)
	}
	sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
	return Hash(sha)
}

func GetHashByte(bytes []byte) (Hash, error) {
	hasher := sha256.New()
	if _, err := hasher.Write(bytes); err != nil {
		return "", err
	}
	sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
	return Hash(sha), nil
}

func MustStruct(i interface{}) Hash {
	hasher := sha256.New()
	if _, err := fmt.Fprintf(hasher, "%v", i); err != nil {
		panic(err)
	}
	sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
	return Hash(sha)
}

func Struct(i interface{}) (Hash, error) {
	hasher := sha256.New()
	if _, err := fmt.Fprintf(hasher, "%v", i); err != nil {
		return "", err
	}
	sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
	return Hash(sha), nil
}
