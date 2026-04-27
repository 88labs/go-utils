package awss3

import (
	"net/url"
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

func (k Key) bucketJoinEscapedAWSString(bucketName BucketName) *string {
	// CopySource is bucket/key, so the slash characters that separate S3 path
	// segments must remain literal even though PathEscape encodes reserved
	// characters inside each segment.
	escapedKey := strings.ReplaceAll(url.PathEscape(k.String()), "%2F", "/")
	return aws.String(bucketName.String() + "/" + escapedKey)
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
