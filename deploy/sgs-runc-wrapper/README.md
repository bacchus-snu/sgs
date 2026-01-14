# SGS OCI Runtime Wrapper

This is an OCI runtime wrapper that intercepts container creation and modifies the OCI spec's `Root.Path` to point to a PVC instead of the overlayfs from image layers.

## How It Works

```text
┌─────────────────────────────────────────────────────────────────┐
│ containerd                                                       │
│   - Prepares OCI bundle with config.json                        │
│   - Root.Path = /path/to/overlayfs (from image layers)          │
└─────────────────────────────────────────────────────────────────┘
                              ↓ calls
┌─────────────────────────────────────────────────────────────────┐
│ sgs-runc-wrapper (THIS)                                         │
│   - Reads config.json                                           │
│   - If sgs.snucse.org/os-volume annotation present:             │
│       - Find beacon mount source (PVC host path)                │
│       - Modify Root.Path = PVC host path                        │
│       - Remove beacon mount from mounts list                    │
│       - Write back config.json                                  │
│   - Exec real runc                                              │
└─────────────────────────────────────────────────────────────────┘
                              ↓ exec
┌─────────────────────────────────────────────────────────────────┐
│ runc                                                            │
│   - Creates container with modified Root.Path                   │
│   - Container rootfs IS the PVC (not overlayfs)                 │
└─────────────────────────────────────────────────────────────────┘
```

## Installation

### 1. Build the Wrapper

```bash
# Static binary (recommended for node deployment)
CGO_ENABLED=0 go build -o sgs-runc-wrapper ./cmd/sgs-runc-wrapper

# Or use make
make sgs-runc-wrapper
```

### 2. Deploy to Nodes

#### Option A: DaemonSet (Recommended)

Use the installer DaemonSet to automatically deploy the wrapper to all nodes:

```bash
# Build and push the container image
docker build -t ghcr.io/bacchus-snu/sgs-runc-wrapper:latest -f deploy/sgs-runc-wrapper/Dockerfile .
docker push ghcr.io/bacchus-snu/sgs-runc-wrapper:latest

# Create namespace and deploy
kubectl create namespace sgs-system
kubectl apply -f deploy/sgs-runc-wrapper/installer-daemonset.yaml
```

The DaemonSet runs an init container that copies the binary to `/usr/local/bin/sgs-runc-wrapper` on each node.

#### Option B: Manual (scp)

Copy the binary to each node manually:

```bash
scp sgs-runc-wrapper root@<node>:/usr/local/bin/
```

### 3. Configure containerd

Edit `/etc/containerd/config.toml` on each node:

```toml
# Add SGS runtime
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.sgs]
  runtime_type = "io.containerd.runc.v2"
  [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.sgs.options]
    BinaryName = "/usr/local/bin/sgs-runc-wrapper"
```

Restart containerd:

```bash
sudo systemctl restart containerd
```

### 4. Create RuntimeClass

```bash
kubectl apply -f deploy/sgs-runc-wrapper/runtimeclass.yaml
```

## Usage

### Pod Spec

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-stateful-container
  annotations:
    sgs.snucse.org/os-volume: "my-pvc"
spec:
  runtimeClassName: sgs
  containers:
    - name: main
      image: busybox:latest
      command: ["/bin/bash"]
      volumeMounts:
        # Beacon mount - any mountPath works, but must be present
        # so the wrapper can find the PVC's host path
        - name: os-volume
          mountPath: /mnt/os
  volumes:
    - name: os-volume
      persistentVolumeClaim:
        claimName: my-pvc
```

See `example-edit-mode.yaml` and `example-run-mode.yaml` for complete examples.

### What Happens

1. **Kubelet** sees the PVC mount and attaches it to the node
2. **containerd** creates an OCI bundle with overlayfs rootfs
3. **sgs-runc-wrapper** intercepts, sees annotation, modifies config.json:
   - `Root.Path` → PVC host path (e.g., `/var/lib/kubelet/pods/.../volumes/...`)
   - Removes beacon mount (it's now the root)
4. **runc** creates container with PVC as actual rootfs

## Debugging

### Check Logs

```bash
# On the node
tail -f /var/log/sgs-runc-wrapper.log
```

### Verify RuntimeClass

```bash
kubectl get runtimeclass sgs
```

### Check Container Rootfs

```bash
# Inside container
cat /proc/1/mountinfo | head -5
# Should show PVC path as root, not overlayfs
```
