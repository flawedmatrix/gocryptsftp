package requester_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRequester(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Requester Suite")
}
