# 노드 상세 조회

`kubectl`을 사용하여 사용 가능한 노드 목록을 조회할 수 있습니다:

```console
$ kubectl get node --selector node-restriction.kubernetes.io/nodegroup=undergraduate
NAME      STATUS   ROLES    AGE    VERSION
ferrari   Ready    <none>   21h    v1.30.9
ford      Ready    <none>   331d   v1.30.4

$ kubectl get node --selector node-restriction.kubernetes.io/nodegroup=graduate
NAME      STATUS   ROLES    AGE    VERSION
bentley   Ready    <none>   215d   v1.30.2
```

컨테이너는 워크스페이스의 노드그룹에 속한 노드 중 하나에 자동으로 할당됩니다.

노드에서 사용 가능한 자원을 조회하려면 `kubectl describe node` 명령어를 사용하세요.

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

위의 출력 예시에서 `bentley`가 가진 총 사용 가능한 자원은 다음과 같습니다:

| 자원     | 할당 가능  | 할당됨 (Requests) |
| -------- | ---------- | ----------------- |
| CPU      | 256 vCPUs  | 0.1 vCPUs         |
| 메모리   | ~1TiB      | 10 MiB            |
| GPU      | 8 GPUs     | 4 GPUs            |
