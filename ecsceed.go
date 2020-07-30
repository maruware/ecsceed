package ecsceed

import (
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/aws-sdk-go/service/ecs"
)

const version = "v0.0.1"

type Service struct {
	srv            ecs.Service
	taskDefinition string
}

type App struct {
	ecs       *ecs.ECS
	cs        ConfigStack
	nameToTd  map[string]ecs.TaskDefinition
	nameToSrv map[string]Service
	region    string
	cluster   string

	Debug bool
}

func NewApp(path string) (*App, error) {
	cs, err := loadConfigStack(path)
	if err != nil {
		return nil, err
	}
	return NewAppWithConfigStack(cs), nil
}

func NewAppWithConfigStack(cs ConfigStack) *App {
	var region string
	var cluster string
	for _, c := range cs {
		if len(c.Region) > 0 {
			region = c.Region
		}
		if len(c.Cluster) > 0 {
			cluster = c.Cluster
		}
	}

	config := aws.NewConfig()
	config.Region = aws.String(region)
	sess := session.New(config)
	c := ecs.New(sess)

	return &App{ecs: c, cs: cs, region: region, cluster: cluster}
}

func (a *App) Name() string {
	return "ecsceed"
}

func (a *App) ResolveConfigStack(additionalParams Params) error {
	params := Params{}
	for _, c := range a.cs {
		for k, v := range c.Params {
			params[k] = v
		}
	}
	for k, v := range additionalParams {
		params[k] = v
	}

	nameToTd := map[string]ecs.TaskDefinition{}
	for _, c := range a.cs {
		for _, tdc := range c.TaskDefinitions {
			var td ecs.TaskDefinition

			if len(tdc.BaseFile) > 0 {
				path, err := filepath.Abs(filepath.Join(c.dir, tdc.BaseFile))
				if err != nil {
					return err
				}
				err = loadAndMatchTmpl(path, params, &td)
				if err != nil {
					return err
				}
			}
			if len(tdc.File) > 0 {
				path, err := filepath.Abs(filepath.Join(c.dir, tdc.File))
				if err != nil {
					return err
				}
				err = loadAndMatchTmpl(path, params, &td)
				if err != nil {
					return err
				}
			}

			// overwrite overlay def
			name := tdc.Name
			nameToTd[name] = td
		}
	}

	nameToSrv := map[string]Service{}
	for _, c := range a.cs {
		for _, sc := range c.Services {
			var srv ecs.Service
			path, err := filepath.Abs(filepath.Join(c.dir, sc.File))
			if err != nil {
				return err
			}

			err = loadAndMatchTmpl(path, params, &srv)
			if err != nil {
				return err
			}

			// overwrite overlay def
			name := sc.Name
			nameToSrv[name] = Service{
				srv:            srv,
				taskDefinition: sc.TaskDefinition,
			}
		}
	}

	a.nameToTd = nameToTd
	a.nameToSrv = nameToSrv

	return nil
}

func (a *App) TaskDefinitionsNum() int {
	return len(a.nameToTd)
}

func (a *App) ServicesNum() int {
	return len(a.nameToSrv)
}

func (a *App) GetTaskDefinition(name string) ecs.TaskDefinition {
	return a.nameToTd[name]
}

func (a *App) GetService(name string) Service {
	return a.nameToSrv[name]
}
