package ecsceed_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/maruware/ecsceed"
	"gopkg.in/yaml.v2"
)

func TestParse(t *testing.T) {
	file, err := os.Open(filepath.Join("test_files", "example1", "base", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}

	var c ecsceed.BaseConfig

	d := yaml.NewDecoder(file)
	if err := d.Decode(&c); err != nil {
		t.Fatal(err)
	}
	if c.Cluster != "my-cluster" {
		t.Errorf("expect cluster is %s but %s", "my-cluster", c.Cluster)
	}
}
