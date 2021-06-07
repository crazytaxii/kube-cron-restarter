# kube-cron-restarter [![Build Status](https://travis-ci.com/crazytaxii/kube-cron-restarter.svg?branch=main)](https://travis-ci.com/crazytaxii/kube-cron-restarter)

The cron restarter is a controller restarts specific pod regularly.

## How to use

### Deploy the controller

```bash
$ kubectl apply -f https://raw.githubusercontent.com/crazytaxii/kube-cron-restarter/main/deploy/cron-restarter.yaml
serviceaccount/cron-restarter created
clusterrole.rbac.authorization.k8s.io/cron-restarter-role created
clusterrolebinding.rbac.authorization.k8s.io/cron-restarter-global created
deployment.apps/cron-restarter created
$ kubectl get deployment cron-restarter -n kube-system
NAME             READY   UP-TO-DATE   AVAILABLE   AGE
cron-restarter   2/2     2            2           85s
$ kubectl get po -n kube-system -l app=cron-restarter
NAME                              READY   STATUS    RESTARTS   AGE
cron-restarter-54c66b94f8-vgvd9   1/1     Running   0          2m32s
cron-restarter-54c66b94f8-vvqc5   1/1     Running   0          2m32s
```

Wrap `cronRestart` as key and cron expression as value in annotations of Deployment or StatefulSet:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
  annotations:
    cronRestart: "* * * * *"
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80
```

The cron restarter controller will create a CronJob with schedule `* * * * *` to restart nginx Pod minutely.

```bash
$ kubectl get po -n kube-system -l app=nginx
NAME                                                             READY   STATUS      RESTARTS   AGE
default-nginx-deployment-deployment-restarter-1622990040-gwbrh   0/1     Completed   0          2m44s
default-nginx-deployment-deployment-restarter-1622990100-d87cl   0/1     Completed   0          104s
default-nginx-deployment-deployment-restarter-1622990160-hmdjm   0/1     Completed   0          44s
```

The CronJob end with restarter has **the same life cycle** as its owner Deployment or StatefulSet. If you delete its owner Deployment/StatefulSet, the CronJob will be deleted automatically.

## How to build

**Please install Go 1.16+ and Docker first.** multi-arch image building needs [buildx](https://docs.docker.com/buildx/working-with-buildx/).

- build binary: `make build`
- build image: `make image`
- build multi-arch image: `make image BUILDX=true`
