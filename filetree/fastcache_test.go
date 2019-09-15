package filetree_test

import (
	"github.com/flawedmatrix/gocryptsftp/filetree"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FastCache", func() {
	var f *filetree.FastCache

	BeforeEach(func() {
		f = filetree.NewFastCache(3)
	})

	expectMappingFound := func(pPath, cPath string) {
		m, found := f.Find(pPath)
		ExpectWithOffset(1, found).To(BeTrue(), "expected key to be found")
		ExpectWithOffset(1, m.PlaintextPath).To(Equal(pPath), "expected requested path to match")
		ExpectWithOffset(1, m.CiphertextPath).To(Equal(cPath), "expected encrypted path to match")
	}

	It("Find returns not found when the item isn't found in cache", func() {
		m, found := f.Find("foo123")
		Expect(found).To(BeFalse())
		Expect(m).To(BeZero())
	})

	It("caches items stored in the cache", func() {
		f.Store("foo123", "bar123")

		for i := 0; i < 5; i++ {
			expectMappingFound("foo123", "bar123")
		}
	})

	It("caches multiple items at once", func() {
		f.Store("foo123", "bar123")
		f.Store("baz123", "baq456")

		for i := 0; i < 5; i++ {
			expectMappingFound("foo123", "bar123")
			expectMappingFound("baz123", "baq456")
		}
	})

	It("evicts the oldest item stored in the cache when storing at capacity", func() {
		f.Store("foo123", "bar123")
		f.Store("baz123", "baq456")
		f.Store("bar123", "foo123")
		f.Store("baq456", "baz123")

		m, found := f.Find("foo123")
		Expect(found).To(BeFalse())
		Expect(m).To(BeZero())

		for i := 0; i < 5; i++ {
			expectMappingFound("baz123", "baq456")
			expectMappingFound("bar123", "foo123")
			expectMappingFound("baq456", "baz123")
		}
	})

	It("updates recently found items to be the most recently used", func() {
		f.Store("foo123", "bar123")
		f.Store("baz123", "baq456")
		f.Store("bar123", "foo123")

		By("using foo123 to make it the most recently used")
		expectMappingFound("foo123", "bar123")

		f.Store("baq456", "baz123")

		// baz123 should be the one evicted now since became the oldest one
		m, found := f.Find("baz123")
		Expect(found).To(BeFalse())
		Expect(m).To(BeZero())

		for i := 0; i < 5; i++ {
			expectMappingFound("foo123", "bar123")
			expectMappingFound("bar123", "foo123")
			expectMappingFound("baq456", "baz123")
		}
	})

	It("allows overwriting cache entries by inserting it again", func() {
		f.Store("foo123", "bar123")
		f.Store("baz123", "baq456")
		f.Store("bar123", "foo123")

		By("overwriting foo123 to make it the most recently used")
		f.Store("foo123", "new-value")

		f.Store("baq456", "baz123")

		// baz123 should be the one evicted now since became the oldest one
		m, found := f.Find("baz123")
		Expect(found).To(BeFalse())
		Expect(m).To(BeZero())

		for i := 0; i < 5; i++ {
			expectMappingFound("foo123", "new-value")
			expectMappingFound("bar123", "foo123")
			expectMappingFound("baq456", "baz123")
		}
	})
})
