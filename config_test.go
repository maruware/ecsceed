package ecsceed

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestParse(t *testing.T) {
	file, err := os.Open(filepath.Join("test_files", "example1", "base", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	var c Config

	d := yaml.NewDecoder(file)
	if err := d.Decode(&c); err != nil {
		t.Fatal(err)
	}
	if c.Cluster != "my-cluster" {
		t.Errorf("expect cluster is %s but %s", "my-cluster", c.Cluster)
	}
}

func TestLoadConfigStack(t *testing.T) {
	path := filepath.Join("test_files", "example1", "overlays", "develop", "config.yml")
	cs, err := loadConfigStack(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 {
		t.Errorf("failed to load config stack")
	}
}
