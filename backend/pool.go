package backend

import (
	"errors"
	"fmt"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type conn struct {
	sshConn  *ssh.Client
	sftpConn *sftp.Client
}

func newConn(remoteAddr string, clientConfig *ssh.ClientConfig) (*conn, error) {
	sshClient, err := ssh.Dial("tcp", remoteAddr, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to remote server %s", err)
	}
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("could not open sftp session to remote %s", err)
	}
	c := &conn{
		sshConn:  sshClient,
		sftpConn: sftpClient,
	}

	return c, nil
}

func (c *conn) Close() {
	if c.sshConn != nil {
		_ = c.sshConn.Close()
	}
	if c.sftpConn != nil {
		_ = c.sftpConn.Close()
	}
}

type pool struct {
	conns chan *conn

	remoteAddr   string
	clientConfig *ssh.ClientConfig
}

func newPool(capacity uint32, remoteAddr string, clientConfig *ssh.ClientConfig) *pool {
	return &pool{
		conns: make(chan *conn, capacity),

		remoteAddr:   remoteAddr,
		clientConfig: clientConfig,
	}
}

func (p *pool) Get() (*conn, error) {
	select {
	case c := <-p.conns:
		if c == nil {
			return nil, errors.New("pool is closed")
		}
		return c, nil
	default:
		return newConn(p.remoteAddr, p.clientConfig)
	}
}

func (p *pool) Put(c *conn) {
	if c == nil {
		return
	}
	select {
	case p.conns <- c:
	default:
		c.Close()
	}
}
