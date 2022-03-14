package devfile

import (
	"path"
	"path/filepath"

	"github.com/devfile/registry-support/index/generator/schema"
	registryLibrary "github.com/devfile/registry-support/registry-library/library"
	"github.com/redhat-developer/alizer/go/pkg/apis/recognizer"
)

func getAlizerDevfileTypes(registryURL string) ([]recognizer.DevFileType, error) {
	types := []recognizer.DevFileType{}
	registryIndex, err := registryLibrary.GetRegistryIndex(registryURL, registryLibrary.RegistryOptions{
		Telemetry: registryLibrary.TelemetryData{},
	}, schema.SampleDevfileType)
	if err != nil {
		return nil, err
	}

	for _, index := range registryIndex {
		types = append(types, recognizer.DevFileType{
			Name:        index.Name,
			Language:    index.Language,
			ProjectType: index.ProjectType,
			Tags:        index.Tags,
		})
	}

	return types, nil
}

// getContext returns the context backtracking from the end of the localpath
func getContext(localpath string, currentLevel int) string {
	context := "./"
	currentPath := localpath
	for i := 0; i < currentLevel; i++ {
		context = path.Join(filepath.Base(currentPath), context)
		currentPath = filepath.Dir(currentPath)
	}

	return context
}
