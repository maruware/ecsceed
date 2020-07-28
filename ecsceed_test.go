package ecsceed_test

import (
	"path/filepath"
	"testing"

	"github.com/maruware/ecsceed"
)

func TestNewApp(t *testing.T) {
	path := filepath.Join("test_files", "example1", "overlays", "develop", "config.yml")
	app, err := ecsceed.NewApp(path)
	if err != nil {
		t.Fatal(err)
	}

	if app.TaskDefinitionsNum() != 2 {
		t.Errorf("expect 2 task definitions")
	}
	if app.ServicesNum() != 2 {
		t.Errorf("expect 2 services")
	}
}
