package envlookup

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/88labs/go-utils/aws/awsconfig"
)

func LookUpString(env string, required bool) string {
	if v, ok := os.LookupEnv(env); ok {
		return v
	}
	if required {
		panic(fmt.Errorf("environment variable is not set to %s", env))
	}
	return ""
}

func LookUpInt(env string) int {
	if envValue, ok := os.LookupEnv(env); ok {
		if v, err := strconv.Atoi(envValue); err != nil {
			panic(err)
		} else {
			return v
		}
	}
	panic(fmt.Errorf("environment variable is not set to %s", env))
}

func LookUpTime(env string) time.Time {
	if envValue, ok := os.LookupEnv(env); ok {
		if v, err := time.Parse(time.RFC3339, envValue); err != nil {
			panic(err)
		} else {
			return v
		}
	}
	panic(fmt.Errorf("environment variable is not set to %s", env))
}

func LookUpRegion(env string) awsconfig.Region {
	v := LookUpString(env, true)
	region, err := awsconfig.ParseRegion(v)
	if err != nil {
		panic(err)
	}
	return region
}
