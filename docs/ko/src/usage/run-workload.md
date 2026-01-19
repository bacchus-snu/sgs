# 워크로드 실행

시작하려면 [예제][examples] 페이지를 확인하세요.

[examples]: run-workload-examples.md

## 저장소

컨테이너의 루트 파일시스템은 임시적이며 컨테이너가 종료되면 모든 것이 사라집니다.
또한 각 컨테이너는 10 GiB의 임시 저장소로 제한됩니다.
워크로드가 이 제한을 초과하면 컨테이너가 자동으로 종료됩니다.

```console
$ # 임시 저장소에 쓰기 작업을 하는 컨테이너 실행
$ kubectl run --image debian:bookworm ephemeral-shell -- bash -c 'cat /dev/zero > /example'

$ # 잠시 후, Pod가 자동으로 종료됨
$ kubectl get events -w | grep ephemeral
0s          Normal    Scheduled             pod/ephemeral-shell           Successfully assigned ws-5y8frda38hqz1/ephemeral-shell to bentley
0s          Normal    Pulled                pod/ephemeral-shell           Container image "debian:bookworm" already present on machine
0s          Normal    Created               pod/ephemeral-shell           Created container ephemeral-shell
0s          Normal    Started               pod/ephemeral-shell           Started container ephemeral-shell
2s          Warning   Evicted               pod/ephemeral-shell           Pod ephemeral local storage usage exceeds the total limit of containers 10Gi.
2s          Normal    Killing               pod/ephemeral-shell           Stopping container ephemeral-shell
```

데이터 저장을 위해 지속 볼륨을 사용하는 것을 강력히 권장합니다.
자세한 내용은 [지속 볼륨 예제][examples-pv]를 참조하세요.

대용량 런타임(예: `pip`를 통한 Python 라이브러리 등)을 설치해야 하는 경우, 종속성이 미리 설치된 이미지를 빌드하는 것을 권장합니다.
이미지를 호스팅하려면 [프라이빗 레지스트리](use-registry.md)를 사용할 수 있습니다.

[examples-pv]: run-workload-examples.md#지속-볼륨
