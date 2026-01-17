# SGS runtime-wrapper

OCI runtime wrapper that enables PVC rootfs replacement for Stateful Containers with support for Nvidia GPU workloads.

## Overview

The `sgs-runtime-wrapper` intercepts OCI runtime calls to modify container root filesystem paths, enabling true rootfs replacement from PersistentVolumeClaims. It supports both standard runc and nvidia-container-runtime through automatic mode detection.

## Features

- **Dual-mode operation**: Automatically detects whether to hijack runc or nvidia-container-runtime
- **Nvidia GPU support**: Transparently works with GPU workloads requiring nvidia-container-runtime
- **PVC rootfs replacement**: Replaces container overlayfs root with PVC host path
- **Annotation-based**: Triggered only when `sgs.snucse.org/os-volume` annotation is present
- **Zero overhead**: Uses `syscall.Exec()` for process replacement
- **Security hardened**: Validates PVC paths, strict permissions, prevents infinite recursion

## How It Works

### Architecture

```
Pod with nvidia.com/gpu + sgs.snucse.org/os-volume annotation
    ↓
containerd → /usr/bin/nvidia-container-runtime (symlink to wrapper)
    ↓
sgs-runtime-wrapper:
  1. Auto-detects nvidia mode from invocation path
  2. Reads OCI config.json from bundle
  3. Finds PVC mount source from kubelet paths
  4. Modifies Root.Path to point to PVC
  5. Removes PVC from mounts list (now the root)
    ↓
exec /usr/bin/nvidia-container-runtime.real
    ↓
nvidia-container-runtime.real → runc (with modified config)
    ↓
Container runs with:
  - GPU access (from nvidia-container-runtime)
  - PVC as rootfs (from wrapper modification)
```

### Mode Detection

The wrapper automatically detects its operating mode:

1. **Manual override**: Set `SGS_WRAPPER_MODE=nvidia` or `SGS_WRAPPER_MODE=runc`
2. **Auto-detection**: Resolves symlinks of executable path:
   - Path contains "nvidia-container-runtime" → nvidia mode
   - Otherwise → runc mode

### Runtime Discovery

**Nvidia mode**:
- Looks for `/usr/bin/nvidia-container-runtime.real` (renamed original)
- Falls back to `/usr/local/bin/nvidia-container-runtime.real`
- If not found, falls back to `/usr/bin/runc` with warning

**Runc mode**:
- Checks `SGS_RUNC_PATH` environment variable
- Uses `exec.LookPath("runc")` with infinite recursion prevention
- Falls back to `/usr/bin/runc`

## Installation

### For Nvidia GPU Nodes (Automated via DaemonSet)

The installation is managed by ArgoCD in the `cd-manifests` repository. The installer DaemonSet:

1. Copies wrapper binary to `/usr/local/bin/sgs-runtime-wrapper`
2. Renames `/usr/bin/nvidia-container-runtime` → `/usr/bin/nvidia-container-runtime.real`
3. Creates symlink: `/usr/bin/nvidia-container-runtime` → `/usr/local/bin/sgs-runtime-wrapper`

**No containerd configuration changes required!**

### Manual Installation

```bash
# Build wrapper
make sgs-runtime-wrapper

# Install binary
sudo cp sgs-runtime-wrapper /usr/local/bin/
sudo chmod +x /usr/local/bin/sgs-runtime-wrapper

# Hijack nvidia-container-runtime
sudo mv /usr/bin/nvidia-container-runtime /usr/bin/nvidia-container-runtime.real
sudo ln -s /usr/local/bin/sgs-runtime-wrapper /usr/bin/nvidia-container-runtime
```

### Binary Location in sgs Image

The `sgs-runtime-wrapper` binary is included in the main `sgs` container image (built via Nix). It is accessible at:

```
/nix/store/<hash>-sgs/bin/sgs-runtime-wrapper
```

To use it in a DaemonSet installer, extract it from the sgs image:

```yaml
initContainers:
  - name: install-runtime-wrapper
    image: ghcr.io/bacchus-snu/sgs:latest
    command: ["/bin/sh", "-c"]
    args:
      - |
        # Find and copy sgs-runtime-wrapper binary from Nix store
        find /nix/store -name sgs-runtime-wrapper -executable -type f \
          -exec cp {} /host/usr/local/bin/sgs-runtime-wrapper \;
        chmod +x /host/usr/local/bin/sgs-runtime-wrapper

        # Hijack nvidia-container-runtime if present
        if [ -f /host/usr/bin/nvidia-container-runtime ]; then
          if [ ! -f /host/usr/bin/nvidia-container-runtime.real ]; then
            mv /host/usr/bin/nvidia-container-runtime \
               /host/usr/bin/nvidia-container-runtime.real
          fi
          ln -sf /usr/local/bin/sgs-runtime-wrapper \
                 /host/usr/bin/nvidia-container-runtime
        fi
    volumeMounts:
      - name: host-bin
        mountPath: /host/usr/bin
      - name: host-local-bin
        mountPath: /host/usr/local/bin
    securityContext:
      privileged: true
```

## Usage

### Pod Specification

Simply add the annotation to enable PVC rootfs replacement. **No `runtimeClassName` needed** when nvidia-container-runtime is hijacked:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: gpu-stateful-container
  annotations:
    sgs.snucse.org/os-volume: "boot-pvc"  # PVC name for rootfs
spec:
  containers:
    - name: main
      image: nvidia/cuda:12.0-base-ubuntu22.04
      command: ["/bin/bash", "-c", "nvidia-smi && sleep infinity"]
      resources:
        limits:
          nvidia.com/gpu: 1  # GPU resource request
      volumeMounts:
        - name: boot-volume
          mountPath: /mnt/boot  # Beacon mount (path doesn't matter)
  volumes:
    - name: boot-volume
      persistentVolumeClaim:
        claimName: boot-pvc  # Must match annotation value
```

**Key points**:
- Annotation `sgs.snucse.org/os-volume` triggers rootfs replacement
- PVC must be mounted somewhere in the pod (beacon mount for discovery)
- GPU resource requests work normally
- No explicit `runtimeClassName` needed (uses default nvidia runtime)

### Without GPU (Traditional runc mode)

For non-GPU nodes using explicit RuntimeClass:

```yaml
spec:
  runtimeClassName: sgs  # Explicit runtime class
  containers:
    - name: main
      # ... rest of spec
```

## Implementation Details

### Code Changes

**File**: [cmd/sgs-runtime-wrapper/main.go](../../cmd/sgs-runtime-wrapper/main.go)

**New constants**:
```go
defaultNvidiaRuntimePath = "/usr/bin/nvidia-container-runtime.real"
envWrapperMode = "SGS_WRAPPER_MODE"
```

**New functions**:
- `detectWrapperMode()`: Auto-detects nvidia vs runc mode from executable path
- `getRuntimePath()`: Replaces `getRuncPath()`, supports both modes

**Modified behavior**:
- Line 115: Mode detection at startup
- Line 111-132: Nvidia runtime discovery with fallback
- Line 150: Correct argv[0] based on runtime type (`nvidia-container-runtime.real` vs `runc`)

### Security Considerations

**Preserved from original**:
- PVC path validation: Must be in `/var/lib/kubelet/pods/`
- File permissions: config.json and logs use 0600
- Strict PVC name matching: Prevents directory traversal
- Annotation-based trigger: Opt-in, not automatic

**Additional for nvidia hijacking**:
- Symlink validation during installation
- Backup of original runtime (`.real` suffix)
- Fallback to runc if nvidia runtime not found
- Installer checks for existing symlinks to prevent conflicts

## Verification

### Check Installation

```bash
# Verify symlink
ls -la /usr/bin/nvidia-container-runtime*
# Should show:
# lrwxrwxrwx ... /usr/bin/nvidia-container-runtime -> /usr/local/bin/sgs-runtime-wrapper
# -rwxr-xr-x ... /usr/bin/nvidia-container-runtime.real

# Check wrapper binary
ls -la /usr/local/bin/sgs-runtime-wrapper
```

### Check Logs

```bash
# View wrapper logs
sudo cat /var/log/sgs-runtime-wrapper.log | tail -20

# Look for:
# - "sgs-runtime-wrapper started in nvidia mode"
# - "Found real nvidia-container-runtime: /usr/bin/nvidia-container-runtime.real"
# - "Found os-volume annotation: <pvc-name>"
# - "Executing real runtime: /usr/bin/nvidia-container-runtime.real"
```

### Verify Container

```bash
# Inside running container, check rootfs
cat /proc/1/mountinfo | grep "/ /"
# Should show PVC path as root, not overlayfs

# Verify GPU access
nvidia-smi
# Should show GPU devices

# Check that modifications persist
touch /test-file
# Restart pod, verify /test-file still exists
```

## Troubleshooting

### GPU not detected

Check that nvidia-container-runtime is properly symlinked:
```bash
ls -la /usr/bin/nvidia-container-runtime
readlink -f /usr/bin/nvidia-container-runtime
```

### PVC not found

Check annotation matches PVC name exactly:
```bash
kubectl get pvc -n <namespace>
kubectl describe pod <pod-name> -n <namespace> | grep sgs.snucse.org/os-volume
```

View wrapper logs to see mount discovery:
```bash
sudo grep "Could not find mount" /var/log/sgs-runtime-wrapper.log
```

### Infinite recursion detected

Wrapper detects itself via path comparison. If this fails, set explicit path:
```bash
# In DaemonSet or node environment
export SGS_RUNC_PATH=/usr/bin/nvidia-container-runtime.real
```

## Uninstallation

Managed by ArgoCD uninstaller DaemonSet, which:

1. Removes symlink: `/usr/bin/nvidia-container-runtime`
2. Restores original: `/usr/bin/nvidia-container-runtime.real` → `nvidia-container-runtime`
3. Removes wrapper: `/usr/local/bin/sgs-runtime-wrapper`

**Manual uninstallation**:
```bash
sudo rm /usr/bin/nvidia-container-runtime
sudo mv /usr/bin/nvidia-container-runtime.real /usr/bin/nvidia-container-runtime
sudo rm /usr/local/bin/sgs-runtime-wrapper
```

## Environment Variables

- `SGS_WRAPPER_MODE`: Override mode detection (`nvidia` or `runc`)
- `SGS_RUNC_PATH`: Override runtime path (e.g., `/usr/bin/nvidia-container-runtime.real`)

## Logging

Logs to `/var/log/sgs-runtime-wrapper.log` with 0600 permissions.

Log levels:
- `Info`: Normal operation (no annotation found, passthrough)
- `Warning`: Recoverable issues (runtime not found, falling back)
- `Error`: Fatal issues (PVC validation failed, config modification failed)

## References

- OCI Runtime Specification: https://github.com/opencontainers/runtime-spec
- nvidia-container-runtime: https://github.com/NVIDIA/nvidia-container-runtime
- Kubernetes RuntimeClass: https://kubernetes.io/docs/concepts/containers/runtime-class/
