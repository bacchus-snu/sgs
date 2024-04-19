# Run your workload

Check out the [examples][examples] page to get started.

[examples]: run-workload-examples.md

## Storage

Your container's root filesystem is ephemeral and everything on it will be lost
when the container is terminated. Also, each container is limited to 10 GiB of
ephemeral storage. If your workload exceeds this limit, the container will be
automatically terminated.

```console
$ # run a container that writes to the ephemeral storage
$ kubectl run --image debian:bookworm ephemeral-shell -- bash -c 'cat /dev/zero > /example'

$ # after a while, the pod gets killed automatically
$ kubectl get events -w | grep ephemeral
0s          Normal    Scheduled             pod/ephemeral-shell           Successfully assigned ws-5y8frda38hqz1/ephemeral-shell to bentley
0s          Normal    Pulled                pod/ephemeral-shell           Container image "debian:bookworm" already present on machine
0s          Normal    Created               pod/ephemeral-shell           Created container ephemeral-shell
0s          Normal    Started               pod/ephemeral-shell           Started container ephemeral-shell
2s          Warning   Evicted               pod/ephemeral-shell           Pod ephemeral local storage usage exceeds the total limit of containers 10Gi.
2s          Normal    Killing               pod/ephemeral-shell           Stopping container ephemeral-shell
```

We strongly recommend that you use persistent volumes for your data. For
details, see the [persistent volume example][examples-pv].

[examples-pv]: run-workload-examples.md#persistent-volume
