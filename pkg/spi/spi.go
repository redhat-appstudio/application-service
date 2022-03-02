package spi

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/redhat-appstudio/application-service/pkg/devfile"
	"github.com/redhat-appstudio/service-provider-integration-scm-file-retriever/gitfile"
)

type SPI interface {
	GetFileContents(ctx context.Context, namespace string, repoUrl string, filepath string, ref string, callback func(ctx context.Context, url string)) (io.ReadCloser, error)
}

type SPIClient struct {
}

func (s SPIClient) GetFileContents(ctx context.Context, namespace string, repoUrl string, filepath string, ref string, callback func(ctx context.Context, url string)) (io.ReadCloser, error) {
	return gitfile.Default().GetFileContents(ctx, namespace, repoUrl, filepath, ref, callback)
}

func DownloadDevfileUsingSPI(s SPI, ctx context.Context, namespace string, repoURL string, ref string, path string) ([]byte, error) {
	validDevfileLocations := []string{devfile.Devfile, devfile.HiddenDevfile, devfile.HiddenDirDevfile, devfile.HiddenDirHiddenDevfile}

	for _, filename := range validDevfileLocations {
		// Call out to SPI via scm-file-retriever to get the devfile from the given repository
		// Hardcode the callback function to nil - no way for us to handle this right now
		r, err := s.GetFileContents(ctx, namespace, repoURL, filepath.Join("/", path, filename), ref, func(ctx context.Context, url string) {})
		if err == nil {
			devfileBytes, err := io.ReadAll(r)
			if err != nil {
				return nil, err
			}
			return devfileBytes, nil
		}

	}

	return nil, fmt.Errorf("unable to find any devfiles in repo %s", repoURL)
}
