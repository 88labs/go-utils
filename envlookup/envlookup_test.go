package envlookup_test

import (
	"testing"
	"time"

	"github.com/88labs/go-utils/envlookup"
	"github.com/bxcodec/faker/v3"

	"github.com/stretchr/testify/assert"
)

func TestLookUpString(t *testing.T) {
	type Param struct {
		Key      string
		Required bool
	}
	type Want struct {
		Val string
		Err bool
	}
	tests := map[string]struct {
		SetEnv func(t *testing.T) (Param, Want)
	}{
		"required:key exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := faker.Sentence()
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Val: val,
						Err: false,
					}
			},
		},
		"required:key not exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := faker.Sentence()
				t.Setenv(key, val)
				return Param{
						Key:      "NOT_EXIST",
						Required: true,
					}, Want{
						Err: true,
					}
			},
		},
		"not required:key exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := faker.Sentence()
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: false,
					}, Want{
						Val: val,
						Err: false,
					}
			},
		},
		"not required:key not exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := faker.Sentence()
				t.Setenv(key, val)
				return Param{
						Key:      "NOT_EXIST",
						Required: false,
					}, Want{
						Val: "",
						Err: false,
					}
			},
		},
	}

	for n, v := range tests {
		name := n
		tt := v
		t.Run(name, func(t *testing.T) {
			p, want := tt.SetEnv(t)
			if want.Err {
				assert.Panics(t, func() {
					envlookup.LookUpString(p.Key, p.Required)
				})
				return
			}
			got := envlookup.LookUpString(p.Key, p.Required)
			assert.Equal(t, want.Val, got)
		})
	}
}

func TestLookUpInt(t *testing.T) {
	type Param struct {
		Key string
	}
	type Want struct {
		Val int
		Err bool
	}
	tests := map[string]struct {
		SetEnv func(t *testing.T) (Param, Want)
	}{
		"key exists:int": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "1"
				t.Setenv(key, val)
				return Param{
						Key: key,
					}, Want{
						Val: 1,
						Err: false,
					}
			},
		},
		"key exists:int = 0": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "0"
				t.Setenv(key, val)
				return Param{
						Key: key,
					}, Want{
						Val: 0,
						Err: false,
					}
			},
		},
		"key exists:not int": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "hoge"
				t.Setenv(key, val)
				return Param{
						Key: key,
					}, Want{
						Err: true,
					}
			},
		},
		"key not exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "1"
				t.Setenv(key, val)
				return Param{
						Key: "NOT_EXIST",
					}, Want{
						Err: true,
					}
			},
		},
	}

	for n, v := range tests {
		name := n
		tt := v
		t.Run(name, func(t *testing.T) {
			p, want := tt.SetEnv(t)
			if want.Err {
				assert.Panics(t, func() {
					envlookup.LookUpInt(p.Key)
				})
				return
			}
			got := envlookup.LookUpInt(p.Key)
			assert.Equal(t, want.Val, got)
		})
	}
}

func TestLookUpTime(t *testing.T) {
	type Param struct {
		Key string
	}
	type Want struct {
		Val time.Time
		Err bool
	}
	tests := map[string]struct {
		SetEnv func(t *testing.T) (Param, Want)
	}{
		"key exists:Time": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "2022-01-02T03:04:05Z"
				t.Setenv(key, val)
				return Param{
						Key: key,
					}, Want{
						Val: time.Date(2022, 1, 2, 3, 4, 5, 0, time.UTC),
						Err: false,
					}
			},
		},
		"key exists:Date": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "2022-01-02"
				t.Setenv(key, val)
				return Param{
						Key: key,
					}, Want{
						Err: true,
					}
			},
		},
		"key exists:not Time": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "hoge"
				t.Setenv(key, val)
				return Param{
						Key: key,
					}, Want{
						Err: true,
					}
			},
		},
		"key not exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := faker.Sentence()
				t.Setenv(key, val)
				return Param{
						Key: "NOT_EXIST",
					}, Want{
						Err: true,
					}
			},
		},
	}

	for n, v := range tests {
		name := n
		tt := v
		t.Run(name, func(t *testing.T) {
			p, want := tt.SetEnv(t)
			if want.Err {
				assert.Panics(t, func() {
					envlookup.LookUpTime(p.Key)
				})
				return
			}
			got := envlookup.LookUpTime(p.Key)
			assert.Equal(t, want.Val, got)
		})
	}
}
