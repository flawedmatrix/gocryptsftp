package config_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/flawedmatrix/gocryptsftp/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Describe("LoadConfig", func() {
		var (
			jsonBytes      []byte
			configFilePath string
		)

		BeforeEach(func() {
			jsonBytes = []byte(`{
				"ProxyUser": "someuser",
				"ProxyPassword": "somepass",
				"KnownHostsPath": "/path/that/exists",
				"Remote": {
					"Addr": "remote.addr.com:22",
					"FileRoot": "/some/file/root",
					"User": "remote-user",
					"PrivateKeyPath": "/private/key/path"
				}
			}`)
		})

		JustBeforeEach(func() {
			f, err := ioutil.TempFile("", "gocryptfs-config")
			Expect(err).ToNot(HaveOccurred())
			defer f.Close()
			configFilePath = f.Name()
			_, err = f.Write(jsonBytes)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			os.Remove(configFilePath)
		})

		It("loads the config from the given filepath", func() {
			cfg, err := config.LoadConfig(configFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.ProxyUser).To(Equal("someuser"))
		})

		It("returns an error if the file doesn't exist", func() {
			cfg, err := config.LoadConfig("/nonexistent/file")
			Expect(err).To(HaveOccurred())
			Expect(cfg).To(BeNil())
		})

		Context("when the json is invalid", func() {
			BeforeEach(func() {
				jsonBytes = []byte(`{"invalid-json`)
			})

			It("returns an error if the json is invalid", func() {
				cfg, err := config.LoadConfig(configFilePath)
				Expect(err).To(HaveOccurred())
				Expect(cfg).To(BeNil())
			})
		})
	})

	Describe("Validate", func() {
		var cfg *config.Config

		BeforeEach(func() {
			cfg = &config.Config{
				ProxyUser:      "someuser",
				ProxyPassword:  "somepass",
				KnownHostsPath: os.Args[0],
				Remote: config.RemoteConfig{
					Addr:           "remote.addr.com:22",
					FileRoot:       "/some/file/root",
					User:           "remote-user",
					PrivateKeyPath: os.Args[0],
				},
			}
		})

		It("successfully validates on a correctly filled out config", func() {
			Expect(cfg.Validate()).To(Succeed())
		})

		Context("when a required field is missing", func() {
			BeforeEach(func() {
				cfg.ProxyUser = ""
			})

			It("fails validation", func() {
				Expect(cfg.Validate()).To(MatchError(ContainSubstring("ProxyUser")))
			})
		})

		Context("when a required file is present but the path doesn't exist", func() {
			BeforeEach(func() {
				cfg.KnownHostsPath = "/nonexistent"
			})

			It("fails validation", func() {
				Expect(cfg.Validate()).To(MatchError(ContainSubstring("KnownHostsPath")))
			})
		})
	})

	Describe("LoadSSHKey", func() {
		var (
			pwReader *config.FakePasswordReader
		)

		BeforeEach(func() {
			pwReader = config.SetTestPWReader()

			pwReader.ReadPasswordReturns(nil, errors.New("Boom"))
		})

		Context("when the key is unencrypted", func() {
			var (
				cfg         *config.Config
				privKeyPath string
			)
			BeforeEach(func() {
				privKeyPath, _ = generatePrivateKey(false)

				cfg = &config.Config{
					Remote: config.RemoteConfig{
						PrivateKeyPath: privKeyPath,
					},
				}
			})

			AfterEach(func() {
				os.Remove(privKeyPath)
			})

			It("loads unencrypted keys", func() {
				sig, err := cfg.LoadSSHKey()
				Expect(sig).ToNot(BeNil())
				Expect(err).ToNot(HaveOccurred())

				Expect(pwReader.ReadPasswordCallCount()).To(BeZero())
			})

			Context("when the keyfile doesn't exist", func() {
				BeforeEach(func() {
					cfg.Remote.PrivateKeyPath = "/nonexistent"
				})

				It("returns an error", func() {
					sig, err := cfg.LoadSSHKey()
					Expect(sig).To(BeNil())
					Expect(err).To(HaveOccurred())

					Expect(pwReader.ReadPasswordCallCount()).To(BeZero())
				})
			})
		})

		Context("when the key is encrypted", func() {
			var (
				cfg           *config.Config
				privKeyPath   string
				keyPassphrase []byte
			)

			BeforeEach(func() {
				privKeyPath, keyPassphrase = generatePrivateKey(true)

				cfg = &config.Config{
					Remote: config.RemoteConfig{
						PrivateKeyPath: privKeyPath,
					},
				}

				pwReader.ReadPasswordReturns(keyPassphrase, nil)
			})

			AfterEach(func() {
				os.Remove(privKeyPath)
			})

			It("loads the encrypted key", func() {
				sig, err := cfg.LoadSSHKey()
				Expect(sig).ToNot(BeNil())
				Expect(err).ToNot(HaveOccurred())

				Expect(pwReader.ReadPasswordCallCount()).To(Equal(1))
				Expect(pwReader.ReadPasswordArgsForCall(0)).To(MatchRegexp(
					fmt.Sprintf("passphrase.*%s", filepath.Base(privKeyPath)),
				))
			})

			Context("when passphrase is incorrect", func() {
				BeforeEach(func() {
					pwReader.ReadPasswordReturns([]byte("wrong-password"), nil)
				})

				It("returns an invalid password error after 3 attempts", func() {
					sig, err := cfg.LoadSSHKey()
					Expect(sig).To(BeNil())
					Expect(err).To(MatchError(x509.IncorrectPasswordError))

					Expect(pwReader.ReadPasswordCallCount()).To(Equal(3))
				})
			})

			Context("when the passphrase is correct after retrying", func() {
				BeforeEach(func() {
					pwReader.ReadPasswordReturns([]byte("wrong-password"), nil)
					pwReader.ReadPasswordReturnsOnCall(1, keyPassphrase, nil)
				})

				It("loads the encrypted key", func() {
					sig, err := cfg.LoadSSHKey()
					Expect(sig).ToNot(BeNil())
					Expect(err).ToNot(HaveOccurred())

					Expect(pwReader.ReadPasswordCallCount()).To(Equal(2))
				})
			})

			Context("when getting the passphrase returns an error", func() {
				BeforeEach(func() {
					pwReader.ReadPasswordReturns(nil, errors.New("boom"))
				})

				It("returns an error immediately", func() {
					sig, err := cfg.LoadSSHKey()
					Expect(sig).To(BeNil())
					Expect(err).To(MatchError("boom"))

					Expect(pwReader.ReadPasswordCallCount()).To(Equal(1))
				})
			})
		})
	})

	Describe("GetDecryptionPassphrase", func() {
		var (
			cfg      *config.Config
			pwReader *config.FakePasswordReader
		)

		BeforeEach(func() {
			pwReader = config.SetTestPWReader()

			cfg = &config.Config{
				Remote: config.RemoteConfig{
					FileRoot: "/some/file/root",
				},
			}

			pwReader.ReadPasswordReturns([]byte("password"), nil)
		})

		It("prompts for the passphrase and returns the bytes", func() {
			b, err := cfg.GetDecrpytionPassphrase()
			Expect(b).To(Equal([]byte("password")))
			Expect(err).ToNot(HaveOccurred())

			Expect(pwReader.ReadPasswordCallCount()).To(Equal(1))
			Expect(pwReader.ReadPasswordArgsForCall(0)).To(MatchRegexp("passphrase.*/some/file/root"))
		})

		Context("when getting the passphrase returns an error", func() {
			BeforeEach(func() {
				pwReader.ReadPasswordReturns(nil, errors.New("boom"))
			})

			It("returns an error", func() {
				b, err := cfg.GetDecrpytionPassphrase()
				Expect(b).To(BeNil())
				Expect(err).To(MatchError("boom"))
			})
		})
	})
})

func generatePrivateKey(encrypted bool) (keyPath string, passphrase []byte) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 128)
	Expect(err).ToNot(HaveOccurred())

	privDER := x509.MarshalPKCS1PrivateKey(privateKey)

	// pem.Block
	privBlock := &pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDER,
	}

	if encrypted {
		passphrase = []byte(time.Now().String())
		privBlock, err = x509.EncryptPEMBlock(
			rand.Reader,
			privBlock.Type,
			privBlock.Bytes,
			passphrase,
			x509.PEMCipherAES256,
		)
		Expect(err).ToNot(HaveOccurred())
	}

	// Private key in PEM format
	privatePEM := pem.EncodeToMemory(privBlock)

	keyFile, err := ioutil.TempFile("", "config-test-priv-key")
	Expect(err).ToNot(HaveOccurred())
	defer keyFile.Close()
	keyPath = keyFile.Name()
	_, err = keyFile.Write(privatePEM)
	Expect(err).ToNot(HaveOccurred())
	return keyPath, passphrase
}
