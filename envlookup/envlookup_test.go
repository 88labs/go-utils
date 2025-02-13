package envlookup_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-faker/faker/v4"
	"gotest.tools/assert"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/envlookup"
)

func TestMust(t *testing.T) {
	type Param struct {
		Key      string
		Required bool
	}
	type Want struct {
		Val string
		Err error
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
						Err: nil,
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
						Err: errors.New("environment variable is not set to " + "NOT_EXIST"),
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
						Err: nil,
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
						Err: nil,
					}
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			p, want := tt.SetEnv(t)
			fnRecover := func(p Param) (v any, err error) {
				defer func() {
					if r := recover(); r != nil {
						err = r.(error)
						return
					}
				}()
				ret := envlookup.Must(envlookup.LookUpString(p.Key, p.Required))
				return ret, nil
			}
			got, err := fnRecover(p)
			if want.Err != nil {
				assert.Error(t, err, want.Err.Error())
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, want.Val, got)
		})
	}
}

func TestLookUpString(t *testing.T) {
	type Param struct {
		Key      string
		Required bool
	}
	type Want struct {
		Val string
		Err error
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
						Err: nil,
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
						Err: errors.New("environment variable is not set to " + "NOT_EXIST"),
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
						Err: nil,
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
						Err: nil,
					}
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			p, want := tt.SetEnv(t)
			got, err := envlookup.LookUpString(p.Key, p.Required)
			if want.Err != nil {
				assert.Error(t, err, want.Err.Error())
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, want.Val, got)
		})
	}
}

func TestLookUpStringSlice(t *testing.T) {
	type Param struct {
		Key      string
		Sep      string
		Required bool
	}
	type Want struct {
		Val []string
		Err error
	}
	tests := map[string]struct {
		SetEnv func(t *testing.T) (Param, Want)
	}{
		"required:key exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val1 := faker.Sentence()
				val2 := faker.Sentence()
				t.Setenv(key, strings.Join([]string{val1, val2}, ","))
				return Param{
						Key:      key,
						Sep:      ",",
						Required: true,
					}, Want{
						Val: []string{val1, val2},
						Err: nil,
					}
			},
		},
		"required:sep:|": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val1 := faker.Sentence()
				val2 := faker.Sentence()
				t.Setenv(key, strings.Join([]string{val1, val2}, "|"))
				return Param{
						Key:      key,
						Sep:      "|",
						Required: true,
					}, Want{
						Val: []string{val1, val2},
						Err: nil,
					}
			},
		},
		"required:key not exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val1 := faker.Sentence()
				val2 := faker.Sentence()
				t.Setenv(key, strings.Join([]string{val1, val2}, ","))
				return Param{
						Key:      "NOT_EXIST",
						Sep:      ",",
						Required: true,
					}, Want{
						Err: errors.New("environment variable is not set to " + "NOT_EXIST"),
					}
			},
		},
		"not required:key exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val1 := faker.Sentence()
				val2 := faker.Sentence()
				t.Setenv(key, strings.Join([]string{val1, val2}, ","))
				return Param{
						Key:      "NOT_EXIST",
						Sep:      ",",
						Required: false,
					}, Want{
						Val: []string(nil),
						Err: nil,
					}
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			p, want := tt.SetEnv(t)
			got, err := envlookup.LookUpStringSlice(p.Key, p.Sep, p.Required)
			if want.Err != nil {
				assert.Error(t, err, want.Err.Error())
				return
			}
			assert.NilError(t, err)
			assert.DeepEqual(t, want.Val, got)
		})
	}
}

func TestLookUpInt(t *testing.T) {
	type Param struct {
		Key      string
		Required bool
	}
	type Want struct {
		Val int
		Err error
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
						Key:      key,
						Required: true,
					}, Want{
						Val: 1,
						Err: nil,
					}
			},
		},
		"key exists:int = 0": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "0"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Val: 0,
						Err: nil,
					}
			},
		},
		"key exists:not int": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "hoge"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Err: errors.New(`strconv.Atoi: parsing "` + val + `": invalid syntax`),
					}
			},
		},
		"not required:key not exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "1"
				t.Setenv(key, val)
				return Param{
						Key:      "NOT_EXIST",
						Required: false,
					}, Want{
						Val: 0,
						Err: nil,
					}
			},
		},
		"required:key not exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "1"
				t.Setenv(key, val)
				return Param{
						Key:      "NOT_EXIST",
						Required: true,
					}, Want{
						Err: errors.New("environment variable is not set to " + "NOT_EXIST"),
					}
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			p, want := tt.SetEnv(t)
			got, err := envlookup.LookUpInt(p.Key, p.Required)
			if want.Err != nil {
				assert.Error(t, err, want.Err.Error())
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, want.Val, got)
		})
	}
}

func TestLookUpTime(t *testing.T) {
	type Param struct {
		Key      string
		Required bool
	}
	type Want struct {
		Val time.Time
		Err error
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
						Key:      key,
						Required: true,
					}, Want{
						Val: time.Date(2022, 1, 2, 3, 4, 5, 0, time.UTC),
						Err: nil,
					}
			},
		},
		"key exists:Date": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "2022-01-02"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Err: errors.New(`parsing time "` + val + `" as "2006-01-02T15:04:05Z07:00": cannot parse "" as "T"`),
					}
			},
		},
		"key exists:not Time": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "hoge"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Err: errors.New(`parsing time "` + val + `" as "2006-01-02T15:04:05Z07:00": cannot parse "hoge" as "2006"`),
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
						Val: time.Time{},
						Err: nil,
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
						Err: errors.New("environment variable is not set to " + "NOT_EXIST"),
					}
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			p, want := tt.SetEnv(t)
			got, err := envlookup.LookUpTime(p.Key, p.Required)
			if want.Err != nil {
				assert.Error(t, err, want.Err.Error())
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, want.Val, got)
		})
	}
}

func TestLookUpDuration(t *testing.T) {
	type Param struct {
		Key      string
		Required bool
	}
	type Want struct {
		Val time.Duration
		Err error
	}
	tests := map[string]struct {
		SetEnv func(t *testing.T) (Param, Want)
	}{
		"key exists:10s": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "10s"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Val: 10 * time.Second,
						Err: nil,
					}
			},
		},
		"key exists:30m": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "30m"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Val: 30 * time.Minute,
						Err: nil,
					}
			},
		},
		"key exists:not Duration": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "hoge"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Err: errors.New(`time: invalid duration "` + val + `"`),
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
						Val: 0,
						Err: nil,
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
						Err: errors.New("environment variable is not set to " + "NOT_EXIST"),
					}
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			p, want := tt.SetEnv(t)
			got, err := envlookup.LookUpDuration(p.Key, p.Required)
			if want.Err != nil {
				assert.Error(t, err, want.Err.Error())
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, want.Val, got)
		})
	}
}

func TestLookUpRegion(t *testing.T) {
	type Param struct {
		Key      string
		Required bool
	}
	type Want struct {
		Val awsconfig.Region
		Err error
	}
	tests := map[string]struct {
		SetEnv func(t *testing.T) (Param, Want)
	}{
		"key exists:Region": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := awsconfig.RegionOhio
				t.Setenv(key, val.String())
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Val: val,
						Err: nil,
					}
			},
		},
		"key exists:not Region": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "hoge"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Err: errors.New("no supported region [" + val + "]"),
					}
			},
		},
		"not required:key not exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := awsconfig.RegionOhio
				t.Setenv(key, val.String())
				return Param{
						Key:      "NOT_EXIST",
						Required: false,
					}, Want{
						Err: errors.New("no supported region []"),
					}
			},
		},
		"required:key not exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := awsconfig.RegionOhio
				t.Setenv(key, val.String())
				return Param{
						Key:      "NOT_EXIST",
						Required: true,
					}, Want{
						Err: errors.New("environment variable is not set to " + "NOT_EXIST"),
					}
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			p, want := tt.SetEnv(t)
			got, err := envlookup.LookUpRegion(p.Key, p.Required)
			if want.Err != nil {
				assert.Error(t, err, want.Err.Error())
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, want.Val, got)
		})
	}
}

func TestLookUpBool(t *testing.T) {
	type Param struct {
		Key      string
		Required bool
	}
	type Want struct {
		Val bool
		Err error
	}
	tests := map[string]struct {
		SetEnv func(t *testing.T) (Param, Want)
	}{
		"required:parse true": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "true"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Val: true,
						Err: nil,
					}
			},
		},
		"required:parse false": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "false"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Val: false,
						Err: nil,
					}
			},
		},
		"required:parse 1": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "1"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Val: true,
						Err: nil,
					}
			},
		},
		"required:parse 0": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "0"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Val: false,
						Err: nil,
					}
			},
		},
		"required:parse error empty": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := ""
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Err: errors.New("environment variable is not set to " + key + " strconv.ParseBool: parsing \"" + val + "\": invalid syntax"),
					}
			},
		},
		"required:key exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "true"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Val: true,
						Err: nil,
					}
			},
		},
		"required:key exists but parse error": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "aaa"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: true,
					}, Want{
						Err: errors.New("environment variable is not set to " + key + " strconv.ParseBool: parsing \"" + val + "\": invalid syntax"),
					}
			},
		},
		"required:key not exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "true"
				t.Setenv(key, val)
				return Param{
						Key:      "NOT_EXIST",
						Required: true,
					}, Want{
						Err: errors.New("environment variable is not set to NOT_EXIST"),
					}
			},
		},
		"not required:key exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "true"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: false,
					}, Want{
						Val: true,
						Err: nil,
					}
			},
		},
		"not required:key exists but parse error": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "aaa"
				t.Setenv(key, val)
				return Param{
						Key:      key,
						Required: false,
					}, Want{
						Val: false,
						Err: errors.New("environment variable is not set to " + key + " strconv.ParseBool: parsing \"" + val + "\": invalid syntax"),
					}
			},
		},
		"not required:key not exists": {
			SetEnv: func(t *testing.T) (Param, Want) {
				key := faker.UUIDHyphenated()
				val := "true"
				t.Setenv(key, val)
				return Param{
						Key:      "NOT_EXIST",
						Required: false,
					}, Want{
						Val: false,
						Err: nil,
					}
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			p, want := tt.SetEnv(t)
			got, err := envlookup.LookUpBool(p.Key, p.Required)
			if want.Err != nil {
				assert.Error(t, err, want.Err.Error())
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, want.Val, got)
		})
	}
}
