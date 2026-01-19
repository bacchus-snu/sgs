# 접속 설정

## CLI 도구 설치

클러스터에 접속하려면 Kubernetes CLI 도구([`kubectl`][kubectl])와 인증 플러그인([`kubelogin`][kubelogin])을 다운로드하고 설치해야 합니다.
설치 방법은 링크된 페이지를 참조하세요.

[kubectl]: https://kubernetes.io/docs/tasks/tools/
[kubelogin]: https://github.com/int128/kubelogin
[sgs]: https://sgs.snucse.org

## CLI 도구 설치 확인

다음 명령어로 CLI 도구가 제대로 설치되었는지 확인하세요.
버전 번호는 다를 수 있습니다.

```console
$ kubectl version --client
kubectl version --client=true
Client Version: v1.30.1
Kustomize Version: v5.0.4-0.20230601165947-6ce0bf390ce3

$ kubectl oidc-login --version
kubelogin version v1.28.1
```

## kubeconfig 파일 다운로드

SGS [워크스페이스 관리 페이지][sgs]를 열고 워크스페이스로 이동하세요.
페이지 하단의 다운로드 버튼을 클릭하여 kubeconfig 파일을 다운로드합니다.

![kubeconfig 다운로드](images/configure-access/download-kubeconfig.png)

다운로드한 kubeconfig 파일을 기본 위치에 저장하세요:

- **Unix** (Linux, MacOS): `~/.kube/config`
- **Windows**: `%USERPROFILE%\.kube\config`

## 설정 확인

`kubectl auth whoami` 명령어로 모든 것이 올바르게 작동하는지 확인하세요.
자동으로 스누씨 ID에 로그인하는 브라우저 창이 열립니다.
로그인 후 다음과 비슷한 출력이 표시됩니다:

```console
$ kubectl auth whoami
ATTRIBUTE   VALUE
Username    id:yseong
Groups      [id:undergraduate system:authenticated]
```
