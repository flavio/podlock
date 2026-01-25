# PodLock

PodLock enforces fine-grained policies on the binaries contained within a container.
The processes started from the locked binaries are sandboxed, limiting which binaries they can execute and controlling
which parts of the container filesystem they can read from or write to.

This is achieved using the Landlock Linux Security Module.

## Prerequisites

- Kubernetes 1.25 or later
- Nodes with Linux kernel supporting Landlock v3 or higher
- NRI-compatible container runtime (containerd 2.0+ or CRI-O 1.25+)
- cert-manager (for webhook certificates)

For detailed requirements and setup instructions, see the [full documentation](https://flavio.github.io/podlock/).

## Installation

```bash
helm repo add podlock https://flavio.github.io/podlock
helm repo update

helm install podlock podlock/podlock \
  --namespace podlock \
  --create-namespace \
  --wait
```

## Configuration

Key configuration options available in `values.yaml`:

| Parameter                     | Description                                               | Default                     |
| ----------------------------- | --------------------------------------------------------- | --------------------------- |
| `controller.image.repository` | Controller container image repository                     | `flavio/podlock/controller` |
| `controller.image.tag`        | Controller container image tag                            | `v0.0.1`                    |
| `controller.replicas`         | Number of controller replicas                             | `1`                         |
| `controller.resources`        | Controller resource limits and requests                   | See values.yaml             |
| `nri.image.repository`        | NRI plugin container image repository                     | `flavio/podlock/nri`        |
| `nri.image.tag`               | NRI plugin container image tag                            | `v0.0.1`                    |
| `nri.logLevel`                | NRI plugin log level (info, debug, warn, error)           | `info`                      |
| `nri.resources`               | NRI plugin resource limits and requests                   | See values.yaml             |
| `vap.enabled`                 | Enable ValidatingAdmissionPolicy for Pod label protection | `true`                      |

### ValidatingAdmissionPolicy

PodLock includes an optional ValidatingAdmissionPolicy that prevents modification of the `podlock.kubewarden.io/profile` label on running Pods,
ensuring profile bindings remain immutable after Pod creation.

Set `vap.enabled: false` to disable. Requires Kubernetes 1.25 or later.

## Documentation

Visit: **https://flavio.github.io/podlock/**
