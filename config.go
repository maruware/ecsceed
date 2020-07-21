package ecsceed

type ConfigTaskDef struct {
	Name string `yaml:"name"`
	File string `yaml:"file"`
}

type ConfigService struct {
	Name           string `yaml:"name"`
	File           string `yaml:"file"`
	TaskDefinition string `yaml:"task_definition"`
}

type BaseConfig struct {
	Region          string            `yaml:"region"`
	Cluster         string            `yaml:"cluster"`
	Params          map[string]string `yaml:"params"`
	TaskDefinitions []ConfigTaskDef   `yaml:"task_definitions"`
	Services        []ConfigService   `yaml:"services"`
}
