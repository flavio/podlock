# PodLock

PodLock restrict process execution and file access in Kubernetes Pods using the [Landlock](https://landlock.io/) Linux Kernel LSM.

Kubernetes users can define PodLock profiles that specify restrictions for processes running inside containers.
Each profile describes the restrictions to apply to individual binaries within a container.
File system access (read, write, and execute permissions) of individual binaries within a container
can be controlled by PodLock.

For example, given the following profile:

```yaml
apiVersion: podlock.kubewarden.io/v1alpha1
kind: LandlockProfile
metadata:
  name: nginx
  namespace: default
spec:
  profilesByContainer:
    nginx:
      "/usr/sbin/nginx":
        readExec:
          - /lib
          - /lib64
        readOnly:
          - /usr/share/nginx
        readWrite:
          - /tmp
```

This profile will limit the processes started by the `/usr/bin/nginx`, they will be
given read and exec rights to the system libraries (`/lib` and `/lib64`).

Access to `/usr/share/nginx` is going to be read only, while `/tmp` will also be writable.

Landlock works in a "deny all" approach. Because of that, the nginx process will not be
able to read other parts of the filesytem. It won't be able to start other binaries from
the system, unless they are under the lib directories (to which it has no write access).

## Documentation

Full project documentation is available at [https://flavio.github.io/podlock](https://flavio.github.io/podlock).

## Current Limitations

- Updating a profile does not automatically trigger a rollout of the associated pods, potentially requiring manual intervention.
- Currently, no mechanisms are available to observe or log violations that are enforced by PodLock.
- A learning mode to assist in profile creation and validation is not implemented yet.
