package filetree_test

import (
	"testing"

	"github.com/flawedmatrix/gocryptsftp/filetree"
)

func BenchmarkFastCacheFind(b *testing.B) {
	f := filetree.NewFastCache(3)
	f.Store("foo123", "bar123")
	f.Store("baz123", "baq456")
	f.Store("bar123", "foo123")

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, found1 := f.Find("foo123")
		_, found2 := f.Find("baz123")
		_, found3 := f.Find("bar123")
		if !found1 || !found2 || !found3 {
			b.Fail()
		}
	}
}

func BenchmarkFastCacheStore(b *testing.B) {
	f := filetree.NewFastCache(3)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		f.Store("foo123", "bar123")
		_, found1 := f.Find("foo123")
		f.Store("foo124", "bar123")
		_, found2 := f.Find("foo124")
		f.Store("foo125", "bar123")
		_, found3 := f.Find("foo125")
		f.Store("foo126", "bar123")
		_, found4 := f.Find("foo126")
		if !found1 || !found2 || !found3 || !found4 {
			b.Fail()
		}
	}
}
