# Configure access

## Install CLI tools

In order to access our cluster, you need to download and install Kubernetes CLI
tooling ([`kubectl`][kubectl]) and the authentication plugin
([`kubelogin`][kubelogin]). Refer to the linked pages for installation
instructions.

[kubectl]: https://kubernetes.io/docs/tasks/tools/
[kubelogin]: https://github.com/int128/kubelogin
[sgs]: https://sgs.snucse.org

## Verify CLI tool installation

Verify your CLI tools were installed with the following commands. Your specific
version numbers may differ.

```console
$ kubectl version --client
kubectl version --client=true
Client Version: v1.30.1
Kustomize Version: v5.0.4-0.20230601165947-6ce0bf390ce3

$ kubectl oidc-login --version
kubelogin version v1.28.1
```

## Download the kubeconfig file

Open the SGS [workspace management page][sgs] and navigate to your workspace.
Click the download button at the bottom of the page to download the kubeconfig
file.

![Download kubeconfig](configure-access/download-kubeconfig.png)

Place your downloaded kubeconfig file in the default kubeconfig location

- **Unix** (Linux, MacOS): `~/.kube/config`
- **Windows**: `%USERPROFILE%\.kube\config`

## Verify your configuration

Use the `kubectl auth whoami` command to check everything is working correctly.
It should automatically open a browser window to log in to SNUCSE ID. After
logging in, you should see something similar to the following output:

```console
$ kubectl auth whoami
ATTRIBUTE   VALUE
Username    id:yseong
Groups      [id:undergraduate system:authenticated]
```
