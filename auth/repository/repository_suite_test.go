package repository

import (
	"testing"

	. "github.com/TaylorOno/golandreporter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSessionRepositoryHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Session Repository Suite", []Reporter{NewAutoGolandReporter()})
}
