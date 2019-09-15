package handlers

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/flawedmatrix/gocryptsftp/filetree"
	"github.com/pkg/sftp"
)

func DecryptHandler(encryptedRoot string, password []byte, numWorkers int, fsAccessor filetree.FSAccessor) (sftp.Handlers, error) {
	ft, err := filetree.Init(encryptedRoot, password, numWorkers, fsAccessor)
	if err != nil {
		return sftp.Handlers{}, err
	}
	h := &decrypt{ft: ft}
	return sftp.Handlers{
		FileGet:  h,
		FilePut:  h,
		FileCmd:  h,
		FileList: h,
	}, nil
}

type decrypt struct {
	ft *filetree.FileTree
}

func (p *decrypt) Fileread(req *sftp.Request) (io.ReaderAt, error) {
	b, err := p.ft.ReadFile(req.Filepath)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(b), nil
}

func (p *decrypt) Filewrite(req *sftp.Request) (io.WriterAt, error) {
	return nil, fmt.Errorf("Writing not supported for path: %s", req.Filepath)
}

func (p *decrypt) Filecmd(req *sftp.Request) error {
	switch req.Method {
	case "Setstat":
		// Probably will never support
		return fmt.Errorf("Setstat not supported. path: %s", req.Filepath)
	case "Rename":
		return fmt.Errorf("Renaming not supported. path: %s, target: %s", req.Filepath, req.Target)
	case "Rmdir", "Remove":
		return fmt.Errorf("Removing not supported. path: %s", req.Filepath)
	case "Mkdir":
		return fmt.Errorf("Mkdir not supported. path: %s", req.Filepath)
	case "Symlink":
		// Probably will never support
		return fmt.Errorf("Symlink not supported. path: %s, target %s", req.Filepath, req.Target)
	}
	return nil
}

func (p *decrypt) Filelist(req *sftp.Request) (sftp.ListerAt, error) {
	switch req.Method {
	case "List":
		fileList, err := p.ft.ReadDir(req.Filepath)
		if err != nil {
			return nil, err
		}
		return listerat(fileList), nil
	case "Stat":
		fileInfo, err := p.ft.Stat(req.Filepath)
		if err != nil {
			return nil, err
		}
		return listerat([]os.FileInfo{fileInfo}), nil
	case "Readlink":
		return nil, fmt.Errorf("ReadLink not supported. path: %s", req.Filepath)
	}
	return nil, nil
}
