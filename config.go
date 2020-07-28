package ecsceed

import (
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type ConfigTaskDef struct {
	Name     string `yaml:"name"`
	BaseFile string `yaml:"base_file"`
	File     string `yaml:"file"`
}

type ConfigService struct {
	Name           string `yaml:"name"`
	File           string `yaml:"file"`
	TaskDefinition string `yaml:"task_definition"`
}

type Config struct {
	Region          string          `yaml:"region"`
	Cluster         string          `yaml:"cluster"`
	Params          Params          `yaml:"params"`
	TaskDefinitions []ConfigTaskDef `yaml:"task_definitions"`
	Services        []ConfigService `yaml:"services"`
	Base            string          `yaml:"base"`

	dir string
}

type ConfigStack []Config

func loadConfigStack(path string) (ConfigStack, error) {

	var tmpPath string
	tmpPath = path
	cs := []Config{}
	for {
		f, err := os.Open(tmpPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		var c Config
		if err := parseConfig(f, &c); err != nil {
			return nil, err
		}

		c.dir = filepath.Dir(tmpPath)

		// unshift
		cs = append([]Config{c}, cs...)

		if len(c.Base) > 0 {
			p, err := filepath.Abs(filepath.Join(c.dir, c.Base))
			if err != nil {
				return nil, err
			}
			tmpPath = p
		} else {
			break // reach root
		}
	}

	return cs, nil
}

func parseConfig(r io.Reader, c *Config) error {
	d := yaml.NewDecoder(r)
	if err := d.Decode(&c); err != nil {
		return err
	}
	return nil
}
