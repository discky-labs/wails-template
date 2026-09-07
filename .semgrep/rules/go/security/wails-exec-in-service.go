package fixtures

import (
	"os/exec"
)

type FileService struct{}

func (s *FileService) OpenWith(path string) error {
	// ruleid: go-wails-exec-in-service
	return exec.Command("xdg-open", path).Run()
}

func (s *FileService) OpenWithCtx(path string) error {
	// ruleid: go-wails-exec-in-service
	cmd := exec.CommandContext(nil, "xdg-open", path)

	return cmd.Run()
}

// ok: go-wails-exec-in-service
func (s *FileService) Safe(name string) (string, error) {
	return "hello " + name, nil
}
