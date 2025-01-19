# Examples

## Recommended workflow

To use the available resources efficiently while ensuring your data is safely
preserved, we recommend the following workflow:

1. Create a working pod without GPU resources to work on your code. Ensure you
   are saving your work in a persistent volume. See the [Persistent
   volume][persistent-volume] example.
2. To test and debug your code, use a GPU shell. See the [GPU shell][gpu-shell]
   example, but ensure you are correctly mounting your persistent volume. Please
   try to limit the time the GPU shell is running while not actively using the
   GPU.
3. Once your code is ready, run it as a GPU workload, ensuring it automatically
   exits once the job is complete. See the [GPU workload][gpu-workload] example.

[persistent-volume]: #persistent-volume
[gpu-shell]: #gpu-shell
[gpu-workload]: #gpu-workload

## Simple ephemeral shell

```console
$ kubectl run --rm -it --image debian:bookworm ephemeral-shell -- /bin/bash
If you don't see a command prompt, try pressing enter.
root@ephemeral-shell:/# nproc
256
root@ephemeral-shell:/# exit
exit
Session ended, resume using 'kubectl attach ephemeral-shell -c ephemeral-shell -i -t' command when the pod is running
pod "ephemeral-shell" deleted
```

The shell and any files in its filesystem are deleted immediately upon exit. No
data is preserved.

## Simple persistent shell

<div class="warning">

Your data will **not**  be preserved across restarts. See the next "Persistent
volume" example to preserve data. We cannot recover data lost due to not using
persistent volumes.

</div>

```yaml
# simple-persistent-shell.yaml
apiVersion: v1
kind: Pod
metadata:
  name: simple-persistent-shell
spec:
  restartPolicy: Never
  terminationGracePeriodSeconds: 1
  containers:
    - name: app
      image: debian:bookworm
      command: ['/bin/bash', '-c', 'sleep inf']
```

```console
$ # create the pod
$ kubectl apply -f simple-persistent-shell.yaml
pod/simple-persistent-shell created

$ # open a shell session
$ kubectl exec -it -f simple-persistent-shell.yaml -- bash
root@simple-persistent-shell:/# exit
exit

$ # later, delete the pod
$ kubectl delete -f simple-persistent-shell.yaml
pod "simple-persistent-shell" deleted
```

## Persistent volume

If you want to preserve data across reboots, use a persistent volume.

```yaml
# persistent-volume.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: persistent-volume
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 100Gi
```

<div class="warning">

The volume **cannot** be resized after creation. Ensure that you allocate enough
space for your needs, according to your approved storage quota.

If you need more storage space,

1. Request a temporary storage quota increase.
2. Create a new PersistentVolumeClaim with the new size.
3. Copy your data from the old volume to the new volume.
4. Delete the old PersistentVolumeClaim.

</div>

```yaml
# persistent-volume-shell.yaml
apiVersion: v1
kind: Pod
metadata:
  name: persistent-volume-shell
spec:
  restartPolicy: Never
  volumes:
    - name: my-volume
      persistentVolumeClaim:
        claimName: persistent-volume
  terminationGracePeriodSeconds: 1
  containers:
    - name: app
      image: debian:bookworm
      command: ['/bin/bash', '-c', 'sleep inf']
      volumeMounts:
        - name: my-volume
          mountPath: /data
```

```console
$ # create resources
$ kubectl apply -f persistent-volume.yaml
persistentvolumeclaim/persistent-volume created
$ kubectl apply -f persistent-volume-shell.yaml
pod/persistent-volume-shell created

$ # open a shell session
$ kubectl exec -it -f persistent-volume-shell.yaml -- bash
root@persistent-volume-shell:/# df -h /data
Filesystem      Size  Used Avail Use% Mounted on
/dev/md127p1    100G     0  100G   0% /data
```

In this example, we mount a 100 GiB volume to the `/data` directory. Your data
will be **irrecoverably lost** if the PersistentVolumeClaim is deleted.

## GPU shell

Use this example to spawn an ephemeral shell with access to GPU resources.

<div class="warning">

The example GPU shell will **auto terminate after 25 minutes, DO NOT** use for consistent workloads.
</div>

```yaml
# gpu-shell.yaml
apiVersion: v1
kind: Pod
metadata:
  name: gpu-shell
spec:
  restartPolicy: Never
  terminationGracePeriodSeconds: 1
  containers:
    - name: app
      image: nvcr.io/nvidia/cuda:12.5.0-base-ubuntu22.04
      command: ['/bin/bash', '-c', 'sleep 1500 && echo "Time expired. Exiting..." && exit']
      resources:
        limits:
          nvidia.com/gpu: 4
```

```console
$ kubectl apply -f gpu-shell.yaml
pod/gpu-shell created

$ kubectl exec -it -f gpu-shell.yaml -- bash
root@gpu-shell:/# nvidia-smi
Tue Jun  4 11:55:12 2024
+---------------------------------------------------------------------------------------+
| NVIDIA-SMI 535.161.08             Driver Version: 535.161.08   CUDA Version: 12.5     |
|-----------------------------------------+----------------------+----------------------+
| GPU  Name                 Persistence-M | Bus-Id        Disp.A | Volatile Uncorr. ECC |
| Fan  Temp   Perf          Pwr:Usage/Cap |         Memory-Usage | GPU-Util  Compute M. |
|                                         |                      |               MIG M. |
|=========================================+======================+======================|
|   0  NVIDIA A100-SXM4-40GB          On  | 00000000:07:00.0 Off |                    0 |
| N/A   24C    P0              53W / 400W |      0MiB / 40960MiB |      0%      Default |
|                                         |                      |             Disabled |
+-----------------------------------------+----------------------+----------------------+
|   1  NVIDIA A100-SXM4-40GB          On  | 00000000:0F:00.0 Off |                    0 |
| N/A   22C    P0              51W / 400W |      0MiB / 40960MiB |      0%      Default |
|                                         |                      |             Disabled |
+-----------------------------------------+----------------------+----------------------+
|   2  NVIDIA A100-SXM4-40GB          On  | 00000000:B7:00.0 Off |                    0 |
| N/A   28C    P0              54W / 400W |      0MiB / 40960MiB |      0%      Default |
|                                         |                      |             Disabled |
+-----------------------------------------+----------------------+----------------------+
|   3  NVIDIA A100-SXM4-40GB          On  | 00000000:BD:00.0 Off |                    0 |
| N/A   29C    P0              58W / 400W |      0MiB / 40960MiB |      0%      Default |
|                                         |                      |             Disabled |
+-----------------------------------------+----------------------+----------------------+

+---------------------------------------------------------------------------------------+
| Processes:                                                                            |
|  GPU   GI   CI        PID   Type   Process name                            GPU Memory |
|        ID   ID                                                             Usage      |
|=======================================================================================|
|  No running processes found                                                           |
+---------------------------------------------------------------------------------------+
```

In this example, we create a pod with 4 GPUs attached. These GPU resources are
exclusively allocated to your pod as long as this pod is running.

<div class="warning">

If you allocate GPU resources but let the GPU idle for extended periods of time,
**we will terminate your pod without warning**. Furthermore, your access may be
permanently restricted. We actively monitor GPU utilization and take action if
we detect abuse.

This warning also applies for "guaranteed" CPU or memory quotas.

</div>

## GPU workload

Similar to the GPU shell, but exit (and de-allocate GPU resources) once the
process terminates.

```yaml
# gpu-workload.yaml
apiVersion: v1
kind: Pod
metadata:
  name: gpu-workload
spec:
  terminationGracePeriodSeconds: 1
  restartPolicy: Never
  containers:
    - name: app
      image: nvcr.io/nvidia/cuda:12.5.0-base-ubuntu22.04
      command: ['/bin/bash', '-c', 'nvidia-smi']
      resources:
        limits:
          nvidia.com/gpu: 4
```

Note the `restartPolicy: Never` and the modified `command` lines.

```console
$ # create the pod
$ kubectl apply -f gpu-workload.yaml
pod/gpu-workload created

$ # watch the pod start and eventually exit
$ kubectl get -f gpu-workload.yaml --watch
NAME           READY   STATUS    RESTARTS   AGE
gpu-workload   1/1     Running   0          6s
gpu-workload   0/1     Completed   0          7s
^C

$ # view logs (standard outputs)
$ kubectl logs gpu-workload --follow
Tue Jun  4 12:14:54 2024
+---------------------------------------------------------------------------------------+
| NVIDIA-SMI 535.161.08             Driver Version: 535.161.08   CUDA Version: 12.5     |
|-----------------------------------------+----------------------+----------------------+
| GPU  Name                 Persistence-M | Bus-Id        Disp.A | Volatile Uncorr. ECC |
| Fan  Temp   Perf          Pwr:Usage/Cap |         Memory-Usage | GPU-Util  Compute M. |
|                                         |                      |               MIG M. |
|=========================================+======================+======================|
|   0  NVIDIA A100-SXM4-40GB          On  | 00000000:07:00.0 Off |                    0 |
| N/A   24C    P0              53W / 400W |      0MiB / 40960MiB |      0%      Default |
|                                         |                      |             Disabled |
+-----------------------------------------+----------------------+----------------------+
|   1  NVIDIA A100-SXM4-40GB          On  | 00000000:0F:00.0 Off |                    0 |
| N/A   22C    P0              51W / 400W |      0MiB / 40960MiB |      0%      Default |
|                                         |                      |             Disabled |
+-----------------------------------------+----------------------+----------------------+
|   2  NVIDIA A100-SXM4-40GB          On  | 00000000:B7:00.0 Off |                    0 |
| N/A   28C    P0              54W / 400W |      0MiB / 40960MiB |      0%      Default |
|                                         |                      |             Disabled |
+-----------------------------------------+----------------------+----------------------+
|   3  NVIDIA A100-SXM4-40GB          On  | 00000000:BD:00.0 Off |                    0 |
| N/A   29C    P0              61W / 400W |      0MiB / 40960MiB |      0%      Default |
|                                         |                      |             Disabled |
+-----------------------------------------+----------------------+----------------------+

+---------------------------------------------------------------------------------------+
| Processes:                                                                            |
|  GPU   GI   CI        PID   Type   Process name                            GPU Memory |
|        ID   ID                                                             Usage      |
|=======================================================================================|
|  No running processes found                                                           |
+---------------------------------------------------------------------------------------+

$ # clean up the pod
$ kubectl delete -f gpu-workload.yaml
pod "gpu-workload" deleted
```

Logs may be truncated to save space, and will be permanently deleted if you
delete your pod. If you want to preserve logs, we recommend writing them to a
persistent volume.

Completed pods do not use node resources. Still, it is a good idea to clean up
completed pods you are no longer using, as they can clutter your namespace and
may result in name collisions.
