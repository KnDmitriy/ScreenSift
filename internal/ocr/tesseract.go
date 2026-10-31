package ocr

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Recognizer interface {
	Recognize(ctx context.Context, path string) (string, error)
}

type Tesseract struct {
	binary    string
	languages string
	timeout   time.Duration
}

func NewTesseract(binary, languages string, timeout time.Duration) *Tesseract {
	return &Tesseract{binary: binary, languages: languages, timeout: timeout}
}

func (t *Tesseract) Recognize(parent context.Context, path string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, t.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, t.binary, path, "stdout", "-l", t.languages)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() != nil {
		return "", fmt.Errorf("ocr timeout: %w", ctx.Err())
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("tesseract: %s", msg)
	}
	return stdout.String(), nil
}
