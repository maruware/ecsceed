package ecsceed

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"text/template"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/imdario/mergo"
)

type App struct {
	nameToTd  map[string]ecs.TaskDefinition
	nameToSrv map[string]ecs.Service
}

func loadAndMatchTmpl(file string, params Params, dst interface{}) error {
	tpl, err := template.ParseFiles(file)
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(nil)
	err = tpl.Execute(buf, params)
	if err != nil {
		return err
	}
	d := json.NewDecoder(buf)
	if err := d.Decode(dst); err != nil {
		return err
	}

	return nil
}

func mergeTaskDefinition(p *ecs.TaskDefinition, c ecs.TaskDefinition) error {
	return mergo.Merge(p, c)
}
func mergeService(p *ecs.Service, c ecs.Service) error {
	return mergo.Merge(p, c)
}

func NewApp(path string) (*App, error) {
	cs, err := resolveConfigStack(path)
	if err != nil {
		return nil, err
	}
	return NewAppWithConfigStack(cs)
}

func NewAppWithConfigStack(config ConfigStack) (*App, error) {
	params := Params{}
	for _, c := range config {
		for k, v := range c.Params {
			params[k] = v
		}
	}

	nameToTd := map[string]ecs.TaskDefinition{}
	for _, c := range config {
		for _, tdc := range c.TaskDefinitions {
			var td ecs.TaskDefinition

			if len(tdc.BaseFile) > 0 {
				path, err := filepath.Abs(filepath.Join(c.dir, tdc.BaseFile))
				if err != nil {
					return nil, err
				}
				err = loadAndMatchTmpl(path, params, &td)
				if err != nil {
					return nil, err
				}
			}
			if len(tdc.File) > 0 {
				path, err := filepath.Abs(filepath.Join(c.dir, tdc.File))
				if err != nil {
					return nil, err
				}
				err = loadAndMatchTmpl(path, params, &td)
				if err != nil {
					return nil, err
				}
			}

			// overwrite overlay def
			name := tdc.Name
			nameToTd[name] = td
		}
	}

	nameToSrv := map[string]ecs.Service{}
	for _, c := range config {
		for _, sc := range c.Services {
			var srv ecs.Service
			path, err := filepath.Abs(filepath.Join(c.dir, sc.File))
			if err != nil {
				return nil, err
			}

			err = loadAndMatchTmpl(path, params, &srv)
			if err != nil {
				return nil, err
			}

			// overwrite overlay def
			name := sc.Name
			nameToSrv[name] = srv
		}
	}

	return &App{
		nameToTd:  nameToTd,
		nameToSrv: nameToSrv,
	}, nil
}

func (a *App) TaskDefinitionsNum() int {
	return len(a.nameToTd)
}
func (a *App) ServicesNum() int {
	return len(a.nameToSrv)
}
