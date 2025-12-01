# Contributing

## Run tests

```shell
make test
```

## Lint code

```shell
make lint
```

## Create the development environment

This project requires to be tested using a kubernetes cluster that is not
running inside of a container (see kind, k3d and similar solution).

It's recommended to use [lima](https://lima-vm.io/) with the default `k3s` template.

Currently, Dec 2025, the kernel of the `minikube` guest OS has not been compiled
with Landlock enabled.

```console
limactl start \
  --cpus 4 \
  --memory 16 \
  --name podlock \
  --yes \
  template:k3s
```

Then you can grab the `kubeconfig` using the following command:

```console
export KUBECONFIG=$(limactl list podlock --format 'unix://{{.Dir}}/copied-from-guest/kubeconfig.yaml')
```

## Run the development environment with Tilt

We use [Tilt](https://tilt.dev/) to run a local development environment.
Customize `tilt-settings.yaml` to your needs.

Run tilt:

```shell
tilt up
```
