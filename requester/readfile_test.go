package requester_test

import (
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/flawedmatrix/gocryptsftp/requester"
	"github.com/flawedmatrix/gocryptsftp/requester/requesterfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ReadFile", func() {
	var (
		backend           *requesterfakes.FakeBackend
		expectedFileBytes []byte
		rqtr              *requester.Requester
	)

	BeforeEach(func() {
		expectedFileBytes = []byte("Some File Bytes")

		backend = new(requesterfakes.FakeBackend)
		backend.ReadFileStub = func(path string) ([]byte, error) {
			if path == "/expected/file/path" {
				return expectedFileBytes, nil
			}
			return nil, errors.New("Not found")
		}

		rqtr = requester.New(10, backend, nil)
		rqtr.Start()
	})

	AfterEach(func() {
		rqtr.Stop()
	})

	It("reads the requested file", func() {
		fileBytes, err := rqtr.ReadFile("/expected/file/path")
		Expect(err).NotTo(HaveOccurred())
		Expect(fileBytes).To(Equal(expectedFileBytes))
	})

	Context("when an error occurs while reading", func() {
		It("returns the error", func() {
			fileBytes, err := rqtr.ReadFile("/nonexistent/file/path")
			Expect(err).To(MatchError("Not found"))
			Expect(fileBytes).To(BeNil())
		})

		It("makes new attempts to get the file on each request", func() {
			for i := 0; i < 5; i++ {
				fileBytes, err := rqtr.ReadFile("/nonexistent/file/path")
				Expect(err).To(MatchError("Not found"))
				Expect(fileBytes).To(BeNil())
			}
			Expect(backend.ReadFileCallCount()).To(Equal(5))
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
					fileBytes, err := rqtr.ReadFile("/expected/file/path")
					Expect(err).NotTo(HaveOccurred())
					Expect(fileBytes).To(Equal(expectedFileBytes))
				}()
			}
			wg.Wait()
			Expect(backend.ReadFileCallCount()).To(Equal(1))
		})

		Context("when an error occurs while reading", func() {
			It("eventually makes new attempts to get the file", func() {
				wg := sync.WaitGroup{}
				for i := 0; i < 5; i++ {
					wg.Add(1)
					go func() {
						defer GinkgoRecover()
						defer wg.Done()
						fileBytes, err := rqtr.ReadFile("/nonexistent/file/path")
						// Because of the concurrent nature, the error isn't guaranteed
						// to be exactly what we wanted.
						Expect(err).To(HaveOccurred())
						Expect(fileBytes).To(BeNil())
					}()
				}
				wg.Wait()
				oldCallCount := backend.ReadFileCallCount()
				fileBytes, err := rqtr.ReadFile("/nonexistent/file/path")
				Expect(err).To(MatchError("Not found"))
				Expect(fileBytes).To(BeNil())
				Expect(backend.ReadFileCallCount()).To(BeNumerically(">=", oldCallCount))
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
							fileBytes, err := rqtr.ReadFile("/expected/file/path")
							Expect(err).NotTo(HaveOccurred())
							Expect(fileBytes).To(Equal(expectedFileBytes))
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
