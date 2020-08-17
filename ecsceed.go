package ecsceed

import (
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/aws-sdk-go/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ecs"
)

type Service struct {
	srv            ecs.Service
	taskDefinition string
}

type Definition struct {
	params     Params
	nameToTd   map[string]ecs.TaskDefinition
	nameToSrv  map[string]Service
	region     string
	cluster    string
	namePrefix string
	nameSuffix string
}

type App struct {
	ecs         *ecs.ECS
	cwl         *cloudwatchlogs.CloudWatchLogs
	autoScaling *applicationautoscaling.ApplicationAutoScaling
	cs          ConfigStack

	def Definition

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
	def := Definition{}
	for _, c := range cs {
		if len(c.Region) > 0 {
			def.region = c.Region
		}
		if len(c.Cluster) > 0 {
			def.cluster = c.Cluster
		}
		if len(c.NamePrefix) > 0 {
			def.namePrefix = c.NamePrefix
		}
		if len(c.NameSuffix) > 0 {
			def.nameSuffix = c.NameSuffix
		}
	}

	config := aws.NewConfig()
	config.Region = aws.String(def.region)
	sess := session.New(config)

	return &App{
		ecs:         ecs.New(sess),
		cwl:         cloudwatchlogs.New(sess),
		autoScaling: applicationautoscaling.New(sess),
		cs:          cs,
		def:         def,
	}
}

func (a *App) Name() string {
	return "ecsceed"
}

func cloneEnvironment(o []*ecs.KeyValuePair) []*ecs.KeyValuePair {
	dst := []*ecs.KeyValuePair{}
	for _, e := range o {
		v := ecs.KeyValuePair{
			Name:  aws.String(*e.Name),
			Value: aws.String(*e.Value),
		}
		dst = append(dst, &v)
	}
	return dst
}

func mergeEnvironment(base []*ecs.KeyValuePair, ex []*ecs.KeyValuePair) []*ecs.KeyValuePair {
	dst := base
	for _, p := range ex {
		dup := false
		for _, bp := range base {
			if *bp.Name == *p.Name {
				dup = true
				bp.Value = p.Value
				break
			}
		}
		if !dup {
			dst = append(dst, p)
		}
	}
	return dst
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
			var baseTd ecs.TaskDefinition

			if len(tdc.BaseFile) > 0 {
				path, err := filepath.Abs(filepath.Join(c.dir, tdc.BaseFile))
				if err != nil {
					return err
				}
				err = loadAndMatchTmpl(path, params, &baseTd)
				if err != nil {
					return err
				}
			}

			var td ecs.TaskDefinition = baseTd
			if len(tdc.File) > 0 {
				path, err := filepath.Abs(filepath.Join(c.dir, tdc.File))
				if err != nil {
					return err
				}

				baseEnvironment := map[string][]*ecs.KeyValuePair{}
				for _, bcd := range baseTd.ContainerDefinitions {
					if len(bcd.Environment) == 0 {
						continue
					}

					name := *bcd.Name
					baseEnvironment[name] = cloneEnvironment(bcd.Environment)
				}

				err = loadAndMatchTmpl(path, params, &td)
				if err != nil {
					return err
				}

				for name, envs := range baseEnvironment {
					// merge environment
					if len(envs) == 0 {
						continue
					}
					for _, cd := range td.ContainerDefinitions {
						if *cd.Name == name {
							cd.Environment = mergeEnvironment(envs, cd.Environment)
							break
						}
					}
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

	a.def.params = params
	a.def.nameToTd = nameToTd
	a.def.nameToSrv = nameToSrv

	return nil
}

func (a *App) TaskDefinitionsNum() int {
	return len(a.def.nameToTd)
}

func (a *App) ServicesNum() int {
	return len(a.def.nameToSrv)
}

func (a *App) GetTaskDefinition(name string) ecs.TaskDefinition {
	return a.def.nameToTd[name]
}

func (a *App) GetService(name string) Service {
	return a.def.nameToSrv[name]
}

func (a *App) resolveFullName(name string) string {
	return a.def.namePrefix + name + a.def.nameSuffix
}

func (a *App) resolveKeyName(fullname string) string {
	n := strings.TrimPrefix(fullname, a.def.namePrefix)
	n = strings.TrimSuffix(n, a.def.nameSuffix)
	return n
}
