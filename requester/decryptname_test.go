package requester_test

import (
	"bytes"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/flawedmatrix/gocryptsftp/requester"
	"github.com/flawedmatrix/gocryptsftp/requester/requesterfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DecryptName", func() {
	var (
		decrypter    *requesterfakes.FakeDecrypter
		expectedName string
		rqtr         *requester.Requester
	)

	BeforeEach(func() {
		expectedName = "decrypted"

		decrypter = new(requesterfakes.FakeDecrypter)
		decrypter.DecryptNameStub = func(name string, iv []byte) (string, error) {
			if name == "encrypted12345" && bytes.Equal(iv, []byte("IV")) {
				return expectedName, nil
			}
			return "", errors.New("Error decrypting")
		}

		rqtr = requester.New(10, nil, decrypter)
		rqtr.Start()
	})

	AfterEach(func() {
		rqtr.Stop()
	})

	It("decrypts the given name with the provided IV", func() {
		decryptedName, err := rqtr.DecryptName("encrypted12345", []byte("IV"))
		Expect(err).NotTo(HaveOccurred())
		Expect(decryptedName).To(Equal(expectedName))
	})

	Context("when an error occurs while decrypting", func() {
		It("returns the error", func() {
			decryptedName, err := rqtr.DecryptName("encrypted12345", []byte("Bad IV"))
			Expect(err).To(MatchError("Error decrypting"))
			Expect(decryptedName).To(BeEmpty())
		})

		It("makes new attempts to decrypt the name on each request", func() {
			for i := 0; i < 5; i++ {
				decryptedName, err := rqtr.DecryptName("encrypted12345", []byte("Bad IV"))
				Expect(err).To(MatchError("Error decrypting"))
				Expect(decryptedName).To(BeEmpty())
			}
			Expect(decrypter.DecryptNameCallCount()).To(Equal(5))
		})
	})

	Context("when making multiple concurrent calls with the same name and IV", func() {
		It("should only decrypt the name once", func() {
			wg := sync.WaitGroup{}
			for i := 0; i < 5; i++ {
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					decryptedName, err := rqtr.DecryptName("encrypted12345", []byte("IV"))
					Expect(err).NotTo(HaveOccurred())
					Expect(decryptedName).To(Equal(expectedName))
				}()
			}
			wg.Wait()
			Expect(decrypter.DecryptNameCallCount()).To(Equal(1))
		})

		Context("when an error occurs while decrypting", func() {
			It("eventually makes new attempts to decrypt the name", func() {
				wg := sync.WaitGroup{}
				for i := 0; i < 5; i++ {
					wg.Add(1)
					go func() {
						defer GinkgoRecover()
						defer wg.Done()
						decryptedName, err := rqtr.DecryptName("encrypted12345", []byte("Bad IV"))
						// Because of the concurrent nature, the error isn't guaranteed
						// to be exactly what we wanted.
						Expect(err).To(HaveOccurred())
						Expect(decryptedName).To(BeEmpty())
					}()
				}
				wg.Wait()
				oldCallCount := decrypter.DecryptNameCallCount()
				decryptedName, err := rqtr.DecryptName("encrypted12345", []byte("Bad IV"))
				Expect(err).To(MatchError("Error decrypting"))
				Expect(decryptedName).To(BeEmpty())
				Expect(decrypter.DecryptNameCallCount()).To(BeNumerically(">=", oldCallCount))
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
							decryptedName, err := rqtr.DecryptName("encrypted12345", []byte("IV"))
							Expect(err).NotTo(HaveOccurred())
							Expect(decryptedName).To(Equal(expectedName))
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
