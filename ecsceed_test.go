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
	container := apiTd.ContainerDefinitions[0]

	assert.Equal(t, "/bin/api", *container.Command[0], "bad api command")
	assert.Equal(t, "my-image:latest", *container.Image, "bad api image")

	for _, env := range container.Environment {
		switch *env.Name {
		case "APP_NAME":
			assert.Equal(t, "awesome-name", *env.Value, "bad APP_NAME")
		case "SERVICE":
			assert.Equal(t, "api", *env.Value, "bad SERVICE")
		case "PORT":
			assert.Equal(t, "8080", *env.Value, "bad PORT")
		}
	}
}
