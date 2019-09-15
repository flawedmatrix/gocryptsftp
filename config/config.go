package config

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"syscall"

	"gopkg.in/go-playground/validator.v9"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

type RemoteConfig struct {
	Addr           string `validate:"required"`
	FileRoot       string `validate:"required"`
	User           string `validate:"required"`
	PrivateKeyPath string `validate:"required,file"`
}

type Config struct {
	ProxyUser      string `validate:"required"`
	ProxyPassword  string `validate:"required"`
	KnownHostsPath string `validate:"required,file"`

	Remote RemoteConfig
}

type PasswordReader interface {
	ReadPassword(prompt string) ([]byte, error)
}

var pwReader PasswordReader = stdinPasswordReader{}

type stdinPasswordReader struct{}

func (s stdinPasswordReader) ReadPassword(prompt string) ([]byte, error) {
	fmt.Print(prompt)
	pass, err := terminal.ReadPassword(int(syscall.Stdin))
	fmt.Println("")
	return pass, err
}

func LoadConfig(path string) (*Config, error) {
	configBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := Config{}
	err = json.Unmarshal(configBytes, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	validate := validator.New()
	return validate.Struct(c)
}

func (c *Config) LoadSSHKey() (ssh.Signer, error) {
	privateBytes, err := ioutil.ReadFile(c.Remote.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load private key: %s", err)
	}
	if isPrivateKeyEncrypted(privateBytes) {
		for i := 0; i < 3; i++ {
			prompt := fmt.Sprintf("Enter passphrase for key '%s': ", c.Remote.PrivateKeyPath)
			passphrase, err := pwReader.ReadPassword(prompt)
			if err != nil {
				return nil, err
			}
			signer, err := ssh.ParsePrivateKeyWithPassphrase(privateBytes, []byte(passphrase))
			if err == nil {
				return signer, nil
			} else if err != x509.IncorrectPasswordError {
				return nil, err
			}
		}
		return nil, x509.IncorrectPasswordError
	}

	return ssh.ParsePrivateKey(privateBytes)
}

func isPrivateKeyEncrypted(key []byte) bool {
	block, _ := pem.Decode(key)
	return x509.IsEncryptedPEMBlock(block)
}

func (c *Config) GetDecrpytionPassphrase() ([]byte, error) {
	prompt := fmt.Sprintf("Enter passphrase for gocryptfs root at '%s': ", c.Remote.FileRoot)
	return pwReader.ReadPassword(prompt)
}
