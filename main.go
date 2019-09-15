// An example SFTP server implementation using the golang SSH package.
// Serves the whole filesystem visible to the user, and has a hard-coded username and password,
// so not for real use!
package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"

	"github.com/flawedmatrix/gocryptsftp/backend"
	"github.com/flawedmatrix/gocryptsftp/config"
	"github.com/flawedmatrix/gocryptsftp/handlers"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Based on example server code from golang.org/x/crypto/ssh and server_standalone
func main() {
	var (
		debugStderr bool
		configPath  string
	)

	flag.BoolVar(&debugStderr, "e", false, "debug to stderr")
	flag.StringVar(&configPath, "c", "", "path to program config")
	flag.Parse()

	debugStream := ioutil.Discard
	if debugStderr {
		debugStream = os.Stderr
	}

	logger := log.New(debugStream, "", log.LstdFlags)

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalln("error loading config file:", err)
	}

	// An SSH server is represented by a ServerConfig, which holds
	// certificate details and handles authentication of ServerConns.
	sshConfig := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			// Should use constant-time compare (or better, salt+hash) in
			// a production setting.
			logger.Println("Login:", c.User())
			if c.User() == cfg.ProxyUser && string(pass) == cfg.ProxyPassword {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},
	}

	private, err := cfg.LoadSSHKey()
	if err != nil {
		log.Fatalln("error loading SSH key:", err)
	}

	sshConfig.AddHostKey(private)

	hostKeyCallback, err := knownhosts.New(cfg.KnownHostsPath)
	if err != nil {
		log.Fatalln("error parsing known hosts file key:", err)
	}

	clientConfig := &ssh.ClientConfig{
		User: cfg.Remote.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(private),
		},
		HostKeyCallback: hostKeyCallback,
	}

	decryptPass, err := cfg.GetDecrpytionPassphrase()
	if err != nil {
		log.Fatalln("error getting decryption passphrase", err)
	}

	backendProvider := backend.NewProvider(cfg.Remote.Addr, clientConfig, logger)
	reqHandlers, err := handlers.DecryptHandler(cfg.Remote.FileRoot, decryptPass, 32, backendProvider)
	if err != nil {
		log.Fatal("Failed to init handler", err)
	}

	// Once a ServerConfig has been configured, connections can be
	// accepted.
	listener, err := net.Listen("tcp", "0.0.0.0:9022")
	if err != nil {
		log.Fatal("failed to listen for connection", err)
	}
	log.Printf("Listening on %v\n", listener.Addr())

	for {
		nConn, err := listener.Accept()
		if err != nil {
			logger.Println("failed to accept incoming connection", err)
			continue
		}
		// Before use, a handshake must be performed on the incoming
		// net.Conn.
		_, chans, reqs, err := ssh.NewServerConn(nConn, sshConfig)
		if err != nil {
			logger.Println("failed to handshake", err)
			continue
		}
		logger.Println("SSH server established")

		// The incoming Request channel must be serviced.
		go ssh.DiscardRequests(reqs)

		go handleChannels(chans, reqHandlers, logger)
	}
}

func handleChannels(chans <-chan ssh.NewChannel, reqHandlers sftp.Handlers, logger *log.Logger) {
	// Service the incoming Channel channel in go routine
	for newChannel := range chans {
		logger.Printf("Incoming channel: %s\n", newChannel.ChannelType())
		go handleChannel(newChannel, reqHandlers, logger)
	}
}

func handleChannel(newChannel ssh.NewChannel, reqHandlers sftp.Handlers, logger *log.Logger) {
	// Channels have a type, depending on the application level
	// protocol intended. In the case of an SFTP session, this is "subsystem"
	// with a payload string of "<length=4>sftp"
	if newChannel.ChannelType() != "session" {
		_ = newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
		logger.Printf("Unknown channel type: %s\n", newChannel.ChannelType())
		return
	}
	channel, requests, err := newChannel.Accept()
	if err != nil {
		logger.Println("could not accept channel:", err)
		return
	}
	logger.Println("Channel accepted")

	// Sessions have out-of-band requests such as "shell",
	// "pty-req" and "env".  Here we handle only the
	// "subsystem" request.
	go func(in <-chan *ssh.Request) {
		for req := range in {
			logger.Printf("Request: %v\n", req.Type)
			ok := false
			switch req.Type {
			case "subsystem":
				logger.Printf("Subsystem: %s\n", req.Payload[4:])
				if string(req.Payload[4:]) == "sftp" {
					ok = true
				}
			}
			logger.Printf(" - accepted: %v\n", ok)
			_ = req.Reply(ok, nil)
		}
	}(requests)

	server := sftp.NewRequestServer(channel, reqHandlers)

	if err := server.Serve(); err == io.EOF {
		server.Close()
		logger.Println("sftp client exited session.")
	} else if err != nil {
		logger.Println("sftp server completed with error:", err)
	}
}
