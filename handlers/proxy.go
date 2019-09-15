package handlers

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/pkg/sftp"
)

// Not used, just a reference implementation

func ProxyHandler(remoteClient *sftp.Client) sftp.Handlers {
	h := &proxy{client: remoteClient}
	return sftp.Handlers{
		FileGet:  h,
		FilePut:  h,
		FileCmd:  h,
		FileList: h,
	}
}

type proxy struct {
	client *sftp.Client
}

func (p *proxy) Fileread(req *sftp.Request) (io.ReaderAt, error) {
	file, err := p.client.Open(req.Filepath)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	_, err = file.WriteTo(buf)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(buf.Bytes()), nil
}

func (p *proxy) Filewrite(req *sftp.Request) (io.WriterAt, error) {
	return nil, fmt.Errorf("Writing not supported for path: %s", req.Filepath)
}

func (p *proxy) Filecmd(req *sftp.Request) error {
	switch req.Method {
	case "Setstat":
		return fmt.Errorf("Setstat not supported. path: %s", req.Filepath)
	case "Rename":
		return fmt.Errorf("Renaming not supported. path: %s, target: %s", req.Filepath, req.Target)
	case "Rmdir", "Remove":
		return fmt.Errorf("Removing not supported. path: %s", req.Filepath)
	case "Mkdir":
		return fmt.Errorf("Mkdir not supported. path: %s", req.Filepath)
	case "Symlink":
		return fmt.Errorf("Symlink not supported. path: %s, target %s", req.Filepath, req.Target)
	}
	return nil
}

type listerat []os.FileInfo

// Modeled after strings.Reader's ReadAt() implementation
func (f listerat) ListAt(ls []os.FileInfo, offset int64) (int, error) {
	var n int
	if offset >= int64(len(f)) {
		return 0, io.EOF
	}
	n = copy(ls, f[offset:])
	if n < len(ls) {
		return n, io.EOF
	}
	return n, nil
}

func (p *proxy) Filelist(req *sftp.Request) (sftp.ListerAt, error) {
	switch req.Method {
	case "List":
		fileList, err := p.client.ReadDir(req.Filepath)
		if err != nil {
			return nil, err
		}
		return listerat(fileList), nil
	case "Stat":
		fileInfo, err := p.client.Stat(req.Filepath)
		if err != nil {
			return nil, err
		}
		return listerat([]os.FileInfo{fileInfo}), nil
	case "Readlink":
		target, err := p.client.ReadLink(req.Filepath)
		if err != nil {
			return nil, err
		}
		fileInfo, err := p.client.Stat(target)
		if err != nil {
			return nil, err
		}
		return listerat([]os.FileInfo{fileInfo}), nil
	}
	return nil, nil
}
