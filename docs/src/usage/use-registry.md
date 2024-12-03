# Use the registry

We provide a per-workspace private registry for your convenience. You can access
the registry at [`sgs-registry.snucse.org`](https://sgs-registry.snucse.org).

The registry shares a high-speed network with the cluster, so image pulls from
the registry should be significantly faster than pulling from public registries
such as nvcr or dockerhub.

## Authenticate to the registry

1. Log in to the registry web interface at
   [`sgs-registry.snucse.org`](https://sgs-registry.snucse.org).
2. Click your account name in the top right corner.
3. Click "User Profile".
4. Copy the "CLI secret" from the profile modal.
5. Configure your Docker client to use the registry:
   ```console
   $ docker login sgs-registry.snucse.org -u <username> -p <cli-secret>
   ```

## Push images to the registry

Navigate to your workspace project's "Repositories" tab and refer to the "Push
command" section for instructions on how to push images to the registry.

```console
$ podman tag my-image:latest sgs-registry.snucse.org/ws-5y8frda38hqz1/this/is/my/image:my-tag
$ podman push sgs-registry.snucse.org/ws-5y8frda38hqz1/this/is/my/image:my-tag
```

## Use images from the registry in your workspace

We automatically configure your workspace with the necessary credentials to pull
images from your workspace project.

```console
$ kubectl run --rm -it --image sgs-registry.snucse.org/ws-5y8frda38hqz1/this/is/my/image:my-tag registry-test
```

<div class="warning">

Do not delete automatically configured `sgs-registry` secret. It cannot be
recovered once deleted.

</div>
