# 예제

## 권장 워크플로우

데이터를 안전하게 보존하면서 사용 가능한 자원을 효율적으로 사용하려면 다음 워크플로우를 권장합니다:

1. GPU 자원 없이 작업용 Pod를 생성하여 코드를 작성하세요. 작업은 지속 볼륨에 저장해야 합니다. [지속 볼륨][persistent-volume] 예제를 참조하세요.
2. 코드를 테스트하고 디버그하려면 GPU 셸을 사용하세요. [GPU 셸][gpu-shell] 예제를 참조하되, 지속 볼륨을 올바르게 마운트하세요. GPU를 적극적으로 사용하지 않는 동안에는 GPU 셸 실행 시간을 제한해주세요.
3. 코드가 준비되면 GPU 워크로드로 실행하되, 작업 완료 후 자동으로 종료되도록 하세요. [GPU 워크로드][gpu-workload] 예제를 참조하세요.

[persistent-volume]: #지속-볼륨
[gpu-shell]: #gpu-셸
[gpu-workload]: #gpu-워크로드

## 간단한 임시 셸

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

셸과 파일시스템의 모든 파일은 종료 즉시 삭제됩니다.
데이터가 보존되지 않습니다.

## 간단한 지속 셸

<div class="warning">

재시작 후 데이터가 **보존되지 않습니다**.
데이터를 보존하려면 다음 "지속 볼륨" 예제를 참조하세요.
지속 볼륨을 사용하지 않아 발생한 데이터 손실은 복구할 수 없습니다.

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
$ # Pod 생성
$ kubectl apply -f simple-persistent-shell.yaml
pod/simple-persistent-shell created

$ # 셸 세션 열기
$ kubectl exec -it -f simple-persistent-shell.yaml -- bash
root@simple-persistent-shell:/# exit
exit

$ # 나중에 Pod 삭제
$ kubectl delete -f simple-persistent-shell.yaml
pod "simple-persistent-shell" deleted
```

## 지속 볼륨

재부팅 후에도 데이터를 보존하려면 지속 볼륨을 사용하세요.

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

볼륨은 생성 후 크기를 **변경할 수 없습니다**.
승인된 저장소 할당량에 따라 필요에 맞는 충분한 공간을 할당하세요.

더 많은 저장 공간이 필요한 경우:

1. 임시 저장소 할당량 증가를 요청하세요.
2. 새 크기로 새 PersistentVolumeClaim을 생성하세요.
3. 기존 볼륨에서 새 볼륨으로 데이터를 복사하세요.
4. 기존 PersistentVolumeClaim을 삭제하세요.

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
$ # 리소스 생성
$ kubectl apply -f persistent-volume.yaml
persistentvolumeclaim/persistent-volume created
$ kubectl apply -f persistent-volume-shell.yaml
pod/persistent-volume-shell created

$ # 셸 세션 열기
$ kubectl exec -it -f persistent-volume-shell.yaml -- bash
root@persistent-volume-shell:/# df -h /data
Filesystem      Size  Used Avail Use% Mounted on
/dev/md127p1    100G     0  100G   0% /data
```

이 예제에서는 100 GiB 볼륨을 `/data` 디렉토리에 마운트합니다.
PersistentVolumeClaim이 삭제되면 데이터가 **복구 불가능하게 손실됩니다**.

## GPU 셸

이 예제를 사용하여 GPU 자원에 접근할 수 있는 임시 셸을 생성하세요.

<div class="warning">

예제 GPU 셸은 **25분 후 자동으로 종료되므로, 지속적인 워크로드에 사용하지 마세요.**
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

이 예제에서는 4개의 GPU가 연결된 Pod를 생성합니다.
이 GPU 자원은 Pod가 실행되는 동안 독점적으로 할당됩니다.

<div class="warning">

GPU 자원을 할당했지만 장기간 GPU를 유휴 상태로 두면 **경고 없이 Pod를 종료합니다**.
또한 접근이 영구적으로 제한될 수 있습니다.
GPU 사용률을 적극적으로 모니터링하고 남용이 감지되면 조치를 취합니다.

이 경고는 "보장" CPU 또는 메모리 할당량에도 적용됩니다.

</div>

## GPU 워크로드

GPU 셸과 비슷하지만 프로세스가 종료되면 GPU 자원을 해제합니다.

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

`restartPolicy: Never`와 수정된 `command` 라인을 확인하세요.

```console
$ # Pod 생성
$ kubectl apply -f gpu-workload.yaml
pod/gpu-workload created

$ # Pod가 시작되고 최종적으로 종료되는 것을 확인
$ kubectl get -f gpu-workload.yaml --watch
NAME           READY   STATUS    RESTARTS   AGE
gpu-workload   1/1     Running   0          6s
gpu-workload   0/1     Completed   0          7s
^C

$ # 로그 보기 (표준 출력)
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

$ # Pod 정리
$ kubectl delete -f gpu-workload.yaml
pod "gpu-workload" deleted
```

로그는 공간 절약을 위해 잘릴 수 있으며 Pod를 삭제하면 영구적으로 삭제됩니다.
로그를 보존하려면 지속 볼륨에 작성하는 것을 권장합니다.

완료된 Pod는 노드 자원을 사용하지 않습니다.
그래도 더 이상 사용하지 않는 완료된 Pod를 정리하는 것이 좋습니다.
네임스페이스가 어질러지고 이름 충돌이 발생할 수 있기 때문입니다.
