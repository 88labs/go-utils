package grpc_test

import (
	"testing"

	. "github.com/TaylorOno/golandreporter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGrpcAuth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Service Suite", []Reporter{NewAutoGolandReporter()})
}
