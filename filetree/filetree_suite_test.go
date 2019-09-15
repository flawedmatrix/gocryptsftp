package filetree_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestFiletree(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Filetree Suite")
}
