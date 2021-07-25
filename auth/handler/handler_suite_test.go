package handler

import (
	"testing"

	. "github.com/TaylorOno/golandreporter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAuthHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Auth Handler Suite", []Reporter{NewAutoGolandReporter()})
}
