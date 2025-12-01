# PodLock

PodLock enforces fine-grained policies on the binaries contained within a container. The processes started from the
locked binaries are sandboxed, limiting which binaries they can execute and controlling which parts of the container
filesystem they can read from or can writ to.

This is achieved using the Landlock Linux Security Module.
