# Query node details

Query the list of nodes available to you using `kubectl`:

```console
$ kubectl get node --selector node-restriction.kubernetes.io/nodegroup=undergraduate
NAME      STATUS   ROLES    AGE    VERSION
ferrari   Ready    <none>   21h    v1.30.9
ford      Ready    <none>   331d   v1.30.4

$ kubectl get node --selector node-restriction.kubernetes.io/nodegroup=graduate
NAME      STATUS   ROLES    AGE    VERSION
bentley   Ready    <none>   215d   v1.30.2
```

Your containers will automatically be assigned to one of the nodes your
workspace's nodegroup.

To query the available resources in your node, you can use the `kubectl describe
node` command.

```console
$ kubectl describe node bentley
Name:               bentley
[...]
Allocatable:
  cpu:                256
  ephemeral-storage:  1699582627075
  hugepages-1Gi:      0
  hugepages-2Mi:      0
  memory:             1056508660Ki
  nvidia.com/gpu:     8
  pods:               110
[...]
Allocated resources:
  (Total limits may be over 100 percent, i.e., overcommitted.)
  Resource           Requests   Limits
  --------           --------   ------
  cpu                100m (0%)  64 (25%)
  memory             10Mi (0%)  64Gi (6%)
  ephemeral-storage  0 (0%)     0 (0%)
  hugepages-1Gi      0 (0%)     0 (0%)
  hugepages-2Mi      0 (0%)     0 (0%)
  nvidia.com/gpu     4          4
```

In the above example output, you can see that `bentley` has the following
resources available in total:

| Resource | Allocatable | Allocated (Requests) |
| -------- | ----------- | --------- |
| CPU      | 256 vCPUs   | 0.1 vCPUs |
| Memory   | ~1TiB       | 10 MiB    |
| GPU      | 8 GPUs      | 4 GPUs    |
