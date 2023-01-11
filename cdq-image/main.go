package main

import (
	"os"
	"strconv"
	"strings"

	"github.com/redhat-appstudio/application-service/cdq-image/pkg"
)

func main() {
	// remove the prefix and suffix quotes
	for i := 1; i <= 10; i++ {
		if strings.HasPrefix(os.Args[i], "\"") && strings.HasSuffix(os.Args[i], "\"") && len(os.Args[i]) > 1 {
			os.Args[i] = os.Args[i][1 : len(os.Args[i])-1]
		}
	}
	gitToken := os.Args[1]
	namespace := os.Args[2]
	name := os.Args[3]
	context := os.Args[4]
	devfilePath := os.Args[5]
	URL := os.Args[6]
	Revision := os.Args[7]
	DevfileRegistryURL := os.Args[8]
	isDevfilePresent, _ := strconv.ParseBool(os.Args[9])
	isDockerfilePresent, _ := strconv.ParseBool(os.Args[10])

	pkg.CloneAndAnalyze(gitToken, namespace, name, context, devfilePath, URL, Revision, DevfileRegistryURL, isDevfilePresent, isDockerfilePresent)
}
