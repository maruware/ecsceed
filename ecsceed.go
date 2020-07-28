package ecsceed

import (
	"path/filepath"

	"github.com/aws/aws-sdk-go/service/ecs"
)

type App struct {
	cs        ConfigStack
	nameToTd  map[string]ecs.TaskDefinition
	nameToSrv map[string]ecs.Service
}

func NewApp(path string) (*App, error) {
	cs, err := loadConfigStack(path)
	if err != nil {
		return nil, err
	}
	return NewAppWithConfigStack(cs), nil
}

func NewAppWithConfigStack(cs ConfigStack) *App {
	return &App{cs: cs}
}

func (a *App) ResolveConfigStack(additionalParams Params) error {
	params := additionalParams
	for _, c := range a.cs {
		for k, v := range c.Params {
			params[k] = v
		}
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

	nameToSrv := map[string]ecs.Service{}
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
			nameToSrv[name] = srv
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

func (a *App) GetService(name string) ecs.Service {
	return a.nameToSrv[name]
}
