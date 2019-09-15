package requester_test

import (
	"errors"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/flawedmatrix/gocryptsftp/requester"
	"github.com/flawedmatrix/gocryptsftp/requester/requesterfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ReadDir", func() {
	var (
		backend           *requesterfakes.FakeBackend
		expectedDirectory []os.FileInfo
		rqtr              *requester.Requester
	)

	BeforeEach(func() {
		dir1 := new(requesterfakes.FakeFileInfo)
		dir1.NameReturns("dir1")
		dir1.IsDirReturns(true)

		file1 := new(requesterfakes.FakeFileInfo)
		file1.NameReturns("file1")
		file1.IsDirReturns(true)

		file2 := new(requesterfakes.FakeFileInfo)
		file2.NameReturns("file2")
		file2.IsDirReturns(true)

		file3 := new(requesterfakes.FakeFileInfo)
		file3.NameReturns("file3")
		file3.IsDirReturns(true)

		expectedDirectory = []os.FileInfo{dir1, file1, file2, file3}

		backend = new(requesterfakes.FakeBackend)
		backend.ReadDirStub = func(path string) ([]os.FileInfo, error) {
			if path == "/expected/dir/path" {
				return expectedDirectory, nil
			}
			return nil, errors.New("Not found")
		}

		rqtr = requester.New(10, backend, nil)
		rqtr.Start()
	})

	AfterEach(func() {
		rqtr.Stop()
	})

	It("reads the requested directory", func() {
		directory, err := rqtr.ReadDir("/expected/dir/path")
		Expect(err).NotTo(HaveOccurred())
		Expect(directory).To(Equal(expectedDirectory))
	})

	Context("when an error occurs while reading", func() {
		It("returns the error", func() {
			directory, err := rqtr.ReadDir("/nonexistent/dir/path")
			Expect(err).To(MatchError("Not found"))
			Expect(directory).To(BeNil())
		})

		It("makes new attempts to get the directory on each request", func() {
			for i := 0; i < 5; i++ {
				directory, err := rqtr.ReadDir("/nonexistent/dir/path")
				Expect(err).To(MatchError("Not found"))
				Expect(directory).To(BeNil())
			}
			Expect(backend.ReadDirCallCount()).To(Equal(5))
		})
	})

	Context("when making multiple concurrent calls with the same path", func() {
		It("should only query the backend once", func() {
			wg := sync.WaitGroup{}
			for i := 0; i < 5; i++ {
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					directory, err := rqtr.ReadDir("/expected/dir/path")
					Expect(err).NotTo(HaveOccurred())
					Expect(directory).To(Equal(expectedDirectory))
				}()
			}
			wg.Wait()
			Expect(backend.ReadDirCallCount()).To(Equal(1))
		})

		Context("when an error occurs while reading", func() {
			It("eventually makes new attempts to get the dir", func() {
				wg := sync.WaitGroup{}
				for i := 0; i < 5; i++ {
					wg.Add(1)
					go func() {
						defer GinkgoRecover()
						defer wg.Done()
						directory, err := rqtr.ReadDir("/nonexistent/dir/path")
						// Because of the concurrent nature, the error isn't guaranteed
						// to be exactly what we wanted.
						Expect(err).To(HaveOccurred())
						Expect(directory).To(BeNil())
					}()
				}
				wg.Wait()
				oldCallCount := backend.ReadDirCallCount()
				directory, err := rqtr.ReadDir("/nonexistent/dir/path")
				Expect(err).To(MatchError("Not found"))
				Expect(directory).To(BeNil())
				Expect(backend.ReadDirCallCount()).To(BeNumerically(">=", oldCallCount))
			})
		})

		Context("when the cache is cleared at some random point", func() {
			It("should continue to work", func() {
				wg := sync.WaitGroup{}
				for i := 0; i < 5; i++ {
					wg.Add(1)
					go func() {
						defer GinkgoRecover()
						defer wg.Done()
						for i := 0; i < 5000; i++ {
							directory, err := rqtr.ReadDir("/expected/dir/path")
							Expect(err).NotTo(HaveOccurred())
							Expect(directory).To(Equal(expectedDirectory))
						}
					}()
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-time.After(time.Duration(rand.Int63n(20)) * time.Millisecond)
					rqtr.ClearCache()
				}()

				wg.Wait()
			})
		})
	})
})
