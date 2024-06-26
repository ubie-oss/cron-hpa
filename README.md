# CronHPA

CronHPA was created by [dtaniwaki](https://github.com/dtaniwaki/cron-hpa). We forked because the original version is no longer being actively maintained.

[![Go Reference][godoc-image]][godoc-link]
[![Coverage Status][cov-image]][cov-link]

CronHPA is an operator designed to modify Horizontal Pod Autoscaler (HPA) resources according to specific schedules. This functionality allows for dynamic adjustments, such as reducing the minimum number of replicas during night-time hours and increasing them during day-time hours.

Here's a `CronHPA` example.

```yaml
apiVersion: cron-hpa.ubie-oss.github.com/v1alpha1
kind: CronHorizontalPodAutoscaler
metadata:
  name: cron-hpa-example
spec:
  template:
    spec:
      scaleTargetRef:
        apiVersion: apps/v1
        kind: Deployment
        name: cron-hpa-nginx
      minReplicas: 3
      maxReplicas: 10
      metrics:
      - type: Resource
        resource:
          name: cpu
          target:
            type: Utilization
            averageUtilization: 50
  scheduledPatches:
  - name: daytime
    schedule: "0 8 * * *"
    timezone: "Asia/Tokyo"
  - name: nighttime
    schedule: "0 22 * * *"
    timezone: "Asia/Tokyo"
    patch:
      minReplicas: 1 # Less minimum replicas.
      - type: Resource
        resource:
          name: cpu
          target:
            type: Utilization
            averageUtilization: 70 # More conservative scaling.
```

## How to use CronHPA

### Disable CronHPA temporarily

Mark the target HPA resource as below to temporarily skip getting CronHPA's update.

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  annotations:
    cron-hpa.ubie-oss.github.com/skip: "true"
...
```

## Prerequisites

- [golangci-lint v1.42.1](https://github.com/golangci/golangci-lint)

## Build

Build and load the Docker image to your cluster.

```bash
$ make docker-build

# run a command to load the image to your cluster.
```

If you use a kind cluster, there's a useful shortcut.

```
$ make kind-load
```

## Deployment

Install the CRD to the cluster.

```bash
$ make install
```

Deploy a controller to the cluster.

```bash
$ make deploy
```

## Usage

Now, deploy the samples.

```bash
$ make deploy-samples
```

You will see sample HPA and deployment in the current context, maybe `default` depending on your env. The HPA resource gets updated periodically by the CronHPA.

## Contributing

1. Fork it
2. Create your feature branch (`git checkout -b my-new-feature`)
3. Commit your changes (`git commit -am 'Add some feature'`)
4. Push to the branch (`git push origin my-new-feature`)
5. Create new [Pull Request](../../pull/new/master)

## Copyright

Copyright (c) 2024 Ubie, Inc. See [LICENSE](LICENSE) for details.


[godoc-image]: https://pkg.go.dev/badge/github.com/ubie-oss/cron-hpa.svg
[godoc-link]: https://pkg.go.dev/github.com/ubie-oss/cron-hpa
[cov-image]:   https://coveralls.io/repos/github/ubie-oss/cron-hpa/badge.svg?branch=main
[cov-link]:    https://coveralls.io/github/ubie-oss/cron-hpa?branch=main

