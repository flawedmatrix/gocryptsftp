package filetree

import "os"

type plainFileInfo struct {
	os.FileInfo

	name string
	size int64
}

func (p plainFileInfo) Name() string {
	return p.name
}

func (p plainFileInfo) Size() int64 {
	return p.size
}
