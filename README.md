# ecsceed

ECS config base deployment tool.  
Inspired [ecspresso](https://github.com/kayac/ecspresso) and [kustomize](https://github.com/kubernetes-sigs/kustomize).

It features: 
* Override base config. (mainly for multi stage)
* Extend Task definition. (e.g. cut out common between web server and worker)

**WIP**

## Example

See [test_files/example1](test_files/example1)

## Install

TODO

## Usage

### Deploy

```bash
ecsceed deploy -c overlays/develop/config.yml -p ImageTag=$(git rev-parse HEAD)
```

### Exec

```bash
ecsceed exec -c overlays/develop/config.yml api echo test
```
