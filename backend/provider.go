package backend

import (
	"bytes"
	"log"
	"os"

	"golang.org/x/crypto/ssh"
)

// Provider provides the ability to make requests to a backend through a pool
// of connections. This package does not provide goroutines on top of
// connections; if you make multiple calls to ReadFile in a single goroutine,
// it's basically the same thing as calling ReadFile on a connection in serial.
// What this package does provide is a guarantee that if you call ReadFile from
// multiple goroutines, no two concurrent calls will share the same connection.
type Provider struct {
	p *pool
}

// NewProvider creates a new instance of a Provider
func NewProvider(remoteAddr string, clientConfig *ssh.ClientConfig, logger *log.Logger) *Provider {
	return &Provider{
		p: newPool(32, remoteAddr, clientConfig),
	}
}

// ReadFile acquires a connection from the connection pool and calls ReadFile
// on the acquired SFTP connection.
func (p *Provider) ReadFile(path string) ([]byte, error) {
	c, err := p.p.Get()
	if err != nil {
		return nil, err
	}
	file, err := c.sftpConn.Open(path)
	if err != nil {
		// Test the underlying ssh connection to see if it's still okay.
		_, _, sshErr := c.sshConn.SendRequest("ping", true, nil)
		if sshErr != nil {
			c.Close()
		} else {
			p.p.Put(c)
		}
		return nil, err
	}
	defer p.p.Put(c)
	defer func() { _ = file.Close() }()
	buf := new(bytes.Buffer)
	_, err = file.WriteTo(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ReadDir acquires a connection from the connection pool and calls ReadDir
// on the acquired SFTP connection.
func (p *Provider) ReadDir(path string) ([]os.FileInfo, error) {
	c, err := p.p.Get()
	if err != nil {
		return nil, err
	}
	listing, err := c.sftpConn.ReadDir(path)
	if err != nil {
		// Test the underlying ssh connection to see if it's still okay.
		_, _, sshErr := c.sshConn.SendRequest("ping", true, nil)
		if sshErr != nil {
			c.Close()
		} else {
			p.p.Put(c)
		}
		return nil, err
	}
	p.p.Put(c)
	return listing, nil
}

// Stat acquires a connection from the connection pool and calls Stat
// on the acquired SFTP connection.
func (p *Provider) Stat(path string) (os.FileInfo, error) {
	c, err := p.p.Get()
	if err != nil {
		return nil, err
	}
	stat, err := c.sftpConn.Stat(path)
	if err != nil {
		// Test the underlying ssh connection to see if it's still okay.
		_, _, sshErr := c.sshConn.SendRequest("ping", true, nil)
		if sshErr != nil {
			c.Close()
		} else {
			p.p.Put(c)
		}
		return nil, err
	}
	p.p.Put(c)
	return stat, nil
}
