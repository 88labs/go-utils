package envlookup

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/88labs/go-utils/aws/awsconfig"
)

func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// LookUpString
// Read values from environment variables with string
// required: If true, panic if the environment variable is not set
func LookUpString(env string, required bool) (string, error) {
	if v, ok := os.LookupEnv(env); ok {
		return v, nil
	}
	if required {
		return "", fmt.Errorf("environment variable is not set to %s", env)
	}
	return "", nil
}

// LookUpStringSlice
// Read values from environment variables with []string
// sep: Separator
// required: If true, panic if the environment variable is not set
func LookUpStringSlice(env string, sep string, required bool) ([]string, error) {
	if v, ok := os.LookupEnv(env); ok {
		return strings.Split(v, sep), nil
	}
	if required {
		return nil, fmt.Errorf("environment variable is not set to %s", env)
	}
	return []string(nil), nil
}

// LookUpInt
// Read values from environment variables with int
// required: If true, panic if the environment variable is not set
func LookUpInt(env string, required bool) (int, error) {
	if envValue, ok := os.LookupEnv(env); ok {
		if v, err := strconv.Atoi(envValue); err != nil {
			return 0, err
		} else {
			return v, nil
		}
	}
	if required {
		return 0, fmt.Errorf("environment variable is not set to %s", env)
	}
	return 0, nil
}

// LookUpTime
// Read values from environment variables with time.Time
// required: If true, panic if the environment variable is not set
func LookUpTime(env string, required bool) (time.Time, error) {
	if envValue, ok := os.LookupEnv(env); ok {
		if v, err := time.Parse(time.RFC3339, envValue); err != nil {
			return time.Time{}, err
		} else {
			return v, nil
		}
	}
	if required {
		return time.Time{}, fmt.Errorf("environment variable is not set to %s", env)
	}
	return time.Time{}, nil
}

// LookUpDuration
// Read values from environment variables with time.Duration
// required: If true, panic if the environment variable is not set
func LookUpDuration(env string, required bool) (time.Duration, error) {
	if envValue, ok := os.LookupEnv(env); ok {
		if v, err := time.ParseDuration(envValue); err != nil {
			return 0, err
		} else {
			return v, nil
		}
	}
	if required {
		return 0, fmt.Errorf("environment variable is not set to %s", env)
	}
	return 0, nil
}

// LookUpRegion
// Read values from environment variables with awsconfig.Region
// required: If true, panic if the environment variable is not set
func LookUpRegion(env string, required bool) (awsconfig.Region, error) {
	v, err := LookUpString(env, required)
	if err != nil {
		return "", err
	}
	region, err := awsconfig.ParseRegion(v)
	if err != nil {
		return "", err
	}
	return region, nil
}

// LookUpBool
// Read values from environment variables with bool
// required: If true, panic if the environment variable is not set
func LookUpBool(env string, required bool) (bool, error) {
	if v, ok := os.LookupEnv(env); ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false, fmt.Errorf("environment variable is not set to %s %s", env, err.Error())
		}
		return b, nil
	}
	if required {
		return false, fmt.Errorf("environment variable is not set to %s", env)
	}
	return false, nil
}
