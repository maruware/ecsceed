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

### Write config

config.yml

```yml
region: ap-northeast-1
cluster: my-cluster

params:
  ImageTag: latest
  ExecutionRoleArn: my-task-execution-role-arn
  TaskRoleArn: my-task-role-arn
  ApiTargetGroupArn: my-alb-target-group-arn

task_definitions:
  - name: api
    base_file: common_td.json
    file: api_td.json
  - name: worker
    base_file: common_td.json
    file: worker_td.json
services:
  - name: api
    task_definition: api
    file: api_service.json
  - name: worker
    task_definition: worker
    file: worker_service.json
```

* **params** : define variants for JSON (Task Definition and Service) template.
* **task_definitions** : define Task Definitions
    * **base_file, file** : Task Definition file. file extends base_file.
* **services** : define Services
    * **task_definition** : ref task_definitions.name
    * **file** : service file.

```json
{
    "containerDefinitions": [
      {
        "cpu": 0,
        "environment": [
          {"name": "APP_NAME", "value": "awesome-name"}
        ],
        "essential": true,
        "image": "debian:{{.ImageTag}}",
        "memoryReservation": 1024,
        "mountPoints": [],
        "name": "app",
        "volumesFrom": [],
        "logConfiguration": {
          "logDriver": "awslogs",
          "options": {
            "awslogs-group": "{{.LogGroup}}",
            "awslogs-region": "ap-northeast-1"
          }
        }
      }
    ],
    "executionRoleArn": "{{.ExecutionRoleArn}}",
    "placementConstraints": [],
    "requiresCompatibilities": [
      "EC2"
    ],
    "volumes": []
}
```

JSON template uses [text/template](https://golang.org/pkg/text/template/) module.


### Deploy

```bash
ecsceed deploy -c overlays/develop/config.yml -p ImageTag=$(git rev-parse HEAD)
```

### Exec

```bash
ecsceed exec -c overlays/develop/config.yml api echo test
```
