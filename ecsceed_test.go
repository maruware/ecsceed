package ecsceed_test

import (
	"path/filepath"
	"testing"

	"github.com/maruware/ecsceed"
	"github.com/stretchr/testify/assert"
)

func TestNewApp(t *testing.T) {
	path := filepath.Join("test_files", "example1", "overlays", "develop", "config.yml")
	app, err := ecsceed.NewApp(path)
	if err != nil {
		t.Fatal(err)
	}
	err = app.ResolveConfigStack(ecsceed.Params{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, app.TaskDefinitionsNum(), "bad task definitions num")
	assert.Equal(t, 2, app.ServicesNum(), "bad services num")

	apiTd := app.GetTaskDefinition("API")

	assert.Equal(t, "/bin/api", *apiTd.ContainerDefinitions[0].Command[0], "bad api command")
	assert.Equal(t, "my-image:latest", *apiTd.ContainerDefinitions[0].Image, "bad api image")
}
