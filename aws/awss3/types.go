package awss3

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type BucketName string

func (k BucketName) String() string {
	return string(k)
}

func (k BucketName) AWSString() *string {
	return aws.String(string(k))
}

type Key string

func (k Key) String() string {
	return string(k)
}

func (k Key) AWSString() *string {
	return aws.String(string(k))
}

func (k Key) BucketJoinAWSString(bucketName BucketName) *string {
	return aws.String(path.Join(bucketName.String(), k.String()))
}

func (k Key) Ext() string {
	return strings.ToLower(filepath.Ext(string(k)))
}

type Keys []Key

func NewKeys(keys ...string) Keys {
	ks := make(Keys, len(keys))
	for i, k := range keys {
		ks[i] = Key(k)
	}
	return ks
}

func (ks Keys) Unique() Keys {
	keys := make(Keys, 0, len(ks))
	uniq := make(map[Key]struct{})
	for _, k := range ks {
		if _, ok := uniq[k]; ok {
			continue
		}
		uniq[k] = struct{}{}
		keys = append(keys, k)
	}
	return keys
}

type Objects []types.Object

func (o Objects) Find(key Key) (types.Object, bool) {
	for _, v := range o {
		if v.Key == nil {
			continue
		}
		if *v.Key == key.String() {
			return v, true
		}
	}
	return types.Object{}, false
}
