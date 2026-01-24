// Package main implements the SGS OCI runtime wrapper binary `sgs-runtime-wrapper`.
//
// This program wraps runc or nvidia-container-runtime and intercepts the "create"
// command to modify the OCI spec's Root.Path. This provides true rootfs
// replacement for "Stateful Containers" with support for GPU workloads.
//
// How it works:
//  1. containerd calls this wrapper instead of the real runtime
//  2. Wrapper auto-detects mode (runc or nvidia) based on invocation path
//  3. Wrapper checks if the container has SGS OS volume mount at /sgs-os-volume
//  4. If present, modifies config.json Root.Path to the PVC host path (mount source)
//  5. Calls the real runtime (runc or nvidia-container-runtime.real) with modified config
//
// Dual-Mode Support:
//   - Nvidia mode: Symlink /usr/bin/nvidia-container-runtime → sgs-runtime-wrapper
//     Calls /usr/bin/nvidia-container-runtime.real after modification
//   - Runc mode: Use RuntimeClass with BinaryName = /usr/local/bin/sgs-runtime-wrapper
//     Calls /usr/bin/runc after modification
//
// Installation (Nvidia GPU hijacking):
//  1. Build: go build -o sgs-runtime-wrapper ./cmd/sgs-runtime-wrapper
//  2. Install: cp sgs-runtime-wrapper /usr/local/bin/
//  3. Rename: mv /usr/bin/nvidia-container-runtime /usr/bin/nvidia-container-runtime.real
//  4. Symlink: ln -s /usr/local/bin/sgs-runtime-wrapper /usr/bin/nvidia-container-runtime
//
// For traditional runc hijacking, configure containerd (/etc/containerd/config.toml):
//
//	[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.sgs]
//	  runtime_type = "io.containerd.runc.v2"
//	  [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.sgs.options]
//	    BinaryName = "/usr/local/bin/sgs-runtime-wrapper"
//
// Then in your Pod spec, use:
//
//	spec:
//	  runtimeClassName: sgs
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

// errNoSGSOSVolume is returned when a container doesn't have SGS OS volume mount (expected for non-SGS containers)
var errNoSGSOSVolume = errors.New("no SGS os-volume mount found")

const (
	// defaultRuncPath is the default path to the actual runc binary
	defaultRuncPath = "/usr/bin/runc"

	// defaultNvidiaRuntimePath is the default path to the renamed nvidia-container-runtime
	defaultNvidiaRuntimePath = "/usr/bin/nvidia-container-runtime.real"

	// envRuncPath is the environment variable to override runtime path
	envRuncPath = "SGS_RUNC_PATH"

	// envWrapperMode is the environment variable to override wrapper mode ("runc" or "nvidia")
	envWrapperMode = "SGS_WRAPPER_MODE"

	// sgsOSVolumeMountPath is the guaranteed mount destination for SGS OS volumes.
	// When CLI attaches an os-volume, it is always mounted at this path.
	sgsOSVolumeMountPath = "/sgs-os-volume"

	// kubeletVolumesPrefix is the expected prefix for kubelet-managed PVC paths
	kubeletVolumesPrefix = "/var/lib/kubelet/pods/"

	// overlayfs directory names within PVC
	overlayUpperDir  = "upper"
	overlayWorkDir   = "work"
	overlayMergedDir = "merged"

	// Minimum kernel version for nested overlayfs support (5.11+)
	minKernelMajor = 5
	minKernelMinor = 11
)

// detectWrapperMode determines whether we're hijacking nvidia-container-runtime or runc.
// It checks SGS_WRAPPER_MODE env var first, then auto-detects based on our executable path.
func detectWrapperMode() string {
	// Check manual override first
	if mode := os.Getenv(envWrapperMode); mode != "" {
		log.Printf("Using wrapper mode from %s: %s", envWrapperMode, mode)
		return mode
	}

	// Auto-detect by checking how we were invoked (argv[0])
	// This preserves the symlink name (e.g., nvidia-container-runtime) even when
	// the symlink points to sgs-runtime-wrapper
	invokedAs := filepath.Base(os.Args[0])
	if strings.Contains(invokedAs, "nvidia-container-runtime") {
		log.Printf("Auto-detected nvidia mode (invoked as: %s)", invokedAs)
		return "nvidia"
	}

	log.Printf("Auto-detected runc mode (invoked as: %s)", invokedAs)
	return "runc"
}

// getRuntimePath returns the path to the real OCI runtime (runc or nvidia-container-runtime).
// It checks SGS_RUNC_PATH env var first for explicit override, then auto-detects based on wrapper mode.
func getRuntimePath() string {
	// Explicit override always wins
	if path := os.Getenv(envRuncPath); path != "" {
		log.Printf("Using runtime path from %s: %s", envRuncPath, path)
		return path
	}

	mode := detectWrapperMode()
	log.Printf("Detected wrapper mode: %s", mode)

	if mode == "nvidia" {
		// Find the renamed nvidia-container-runtime
		candidates := []string{
			defaultNvidiaRuntimePath,
			"/usr/local/bin/nvidia-container-runtime.real",
		}

		for _, path := range candidates {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				// Verify it's executable
				if info.Mode()&0111 != 0 {
					log.Printf("Found real nvidia-container-runtime: %s", path)
					return path
				}
			}
		}

		log.Printf("Warning: nvidia-container-runtime.real not found, falling back to runc")
		return defaultRuncPath
	}

	// Original runc discovery logic for runc mode
	if path, err := exec.LookPath("runc"); err == nil {
		// Check if the found path is our own executable (prevent infinite recursion)
		if selfPath, selfErr := os.Executable(); selfErr == nil {
			// Resolve symlinks for accurate comparison
			resolvedPath, _ := filepath.EvalSymlinks(path)
			resolvedSelf, _ := filepath.EvalSymlinks(selfPath)
			if resolvedPath == "" {
				resolvedPath = path
			}
			if resolvedSelf == "" {
				resolvedSelf = selfPath
			}
			if resolvedPath != resolvedSelf {
				log.Printf("Found runc via PATH: %s", path)
				return path
			}
			log.Printf("Warning: LookPath returned our own executable, skipping")
		} else {
			// Can't determine self path, use found path anyway
			log.Printf("Found runc via PATH: %s", path)
			return path
		}
	}

	log.Printf("Using default runc path: %s", defaultRuncPath)
	return defaultRuncPath
}

func main() {
	// Set up logging to a file for debugging (0600 to protect sensitive info)
	logFile, err := os.OpenFile("/var/log/sgs-runtime-wrapper.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err == nil {
		log.SetOutput(logFile)
		defer func() {
			if err := logFile.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "sgs-runtime-wrapper: failed to close log file: %v\n", err)
			}
		}()
	} else {
		fmt.Fprintf(os.Stderr, "sgs-runtime-wrapper: failed to open log file /var/log/sgs-runtime-wrapper.log: %v\n", err)
	}

	args := os.Args[1:]

	log.Printf("sgs-runtime-wrapper called with args: %v", args)

	// Look for "create" command and bundle path
	var bundlePath string
	isCreate := false

	for i, arg := range args {
		if arg == "create" {
			isCreate = true
		}
		if arg == "--bundle" || arg == "-b" {
			if i+1 < len(args) {
				bundlePath = args[i+1]
			}
		}
	}

	// If this is a create command, potentially modify the config
	if isCreate {
		if bundlePath == "" {
			log.Printf("Warning: 'create' command detected but no bundle path found; proceeding without SGS modification")
		} else if err := maybeModifyRootfs(bundlePath); err != nil {
			// Check if this is an expected "no SGS OS volume" case or an actual error
			if errors.Is(err, errNoSGSOSVolume) {
				log.Printf("Info: %v", err)
			} else {
				log.Printf("Error: SGS rootfs modification failed: %v", err)
				// For SGS containers with actual errors, fail fast rather than proceeding with wrong config
				fmt.Fprintf(os.Stderr, "sgs-runtime-wrapper: %v\n", err)
				os.Exit(1)
			}
		}
	}

	// Execute the real OCI runtime (runc or nvidia-container-runtime)
	runtimePath := getRuntimePath()
	argv0 := filepath.Base(runtimePath) // e.g., "nvidia-container-runtime.real" or "runc"
	log.Printf("Executing real runtime: %s %v", runtimePath, args)
	execErr := syscall.Exec(runtimePath, append([]string{argv0}, args...), os.Environ())
	if execErr != nil {
		log.Printf("Failed to exec runtime: %v", execErr)
		// Fallback to exec.Command if syscall.Exec fails
		cmd := exec.Command(runtimePath, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			os.Exit(1)
		}
	}
}

// maybeModifyRootfs checks the OCI spec for SGS OS volume mount and modifies Root.Path if needed.
func maybeModifyRootfs(bundlePath string) error {
	configPath := filepath.Join(bundlePath, "config.json")

	// Read the OCI config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config.json: %w", err)
	}

	var spec specs.Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		return fmt.Errorf("failed to parse config.json: %w", err)
	}

	// Find the SGS OS volume mount by its known destination path (/sgs-os-volume).
	// When CLI attaches an os-volume, it is guaranteed to be mounted at this path.
	pvcHostPath := findSGSOSVolumeMountSource(&spec)

	if pvcHostPath == "" {
		return errNoSGSOSVolume
	}

	log.Printf("PVC host path: %s", pvcHostPath)

	// Validate that the PVC path is within expected kubelet directory to prevent
	// arbitrary host directories from being used as rootfs
	if !strings.HasPrefix(pvcHostPath, kubeletVolumesPrefix) {
		return fmt.Errorf("PVC host path '%s' is not within expected kubelet directory '%s'; "+
			"this may indicate a security issue or misconfiguration", pvcHostPath, kubeletVolumesPrefix)
	}

	// Validate that the PVC path exists and is a directory
	pvcInfo, err := os.Stat(pvcHostPath)
	if err != nil {
		return fmt.Errorf("PVC host path validation failed: %w", err)
	}
	if !pvcInfo.IsDir() {
		return fmt.Errorf("PVC host path is not a directory: %s", pvcHostPath)
	}

	// Store the original root path (container image rootfs) - this will be the lowerdir
	originalRoot := spec.Root.Path
	log.Printf("Original root path (lowerdir): %s", originalRoot)

	// Resolve to absolute path if relative (OCI spec often uses relative "rootfs")
	if !filepath.IsAbs(originalRoot) {
		originalRoot = filepath.Join(bundlePath, originalRoot)
		log.Printf("Resolved to absolute path: %s", originalRoot)
	}

	// Setup overlayfs: image (lowerdir) + PVC (upperdir) = merged view
	mergedPath, err := setupOverlayfs(originalRoot, pvcHostPath)
	if err != nil {
		return fmt.Errorf("overlayfs setup failed: %w", err)
	}

	// Point container root to the merged overlayfs view
	spec.Root.Path = mergedPath
	spec.Root.Readonly = false

	log.Printf("New root path (overlay merged): %s", spec.Root.Path)

	// Add poststop hook to unmount overlayfs when container exits
	// Use lazy unmount (-l) to avoid "device busy" errors
	if spec.Hooks == nil {
		spec.Hooks = &specs.Hooks{}
	}
	spec.Hooks.Poststop = append(spec.Hooks.Poststop, specs.Hook{
		Path: "/bin/umount",
		Args: []string{"umount", "-l", mergedPath},
	})
	log.Printf("Added poststop hook to unmount %s", mergedPath)

	// Add annotations to track the overlay configuration (for debugging)
	if spec.Annotations == nil {
		spec.Annotations = make(map[string]string)
	}
	spec.Annotations["sgs.snucse.org/overlay-lowerdir"] = originalRoot
	spec.Annotations["sgs.snucse.org/overlay-upperdir"] = filepath.Join(pvcHostPath, overlayUpperDir)
	spec.Annotations["sgs.snucse.org/overlay-merged"] = mergedPath

	// Remove the PVC mount from the mounts list since it's now the root.
	// We find it by matching the source path we're using as root.
	// Use EvalSymlinks to normalize paths and handle symlinks/bind mounts.
	normalizedPVCPath, err := filepath.EvalSymlinks(pvcHostPath)
	if err != nil {
		log.Printf("Warning: failed to resolve symlinks for PVC path '%s': %v; using original path", pvcHostPath, err)
		normalizedPVCPath = pvcHostPath
	}
	filteredMounts := make([]specs.Mount, 0, len(spec.Mounts))
	for _, mount := range spec.Mounts {
		normalizedSource, err := filepath.EvalSymlinks(mount.Source)
		if err != nil {
			log.Printf("Warning: failed to resolve symlinks for mount source '%s': %v; using original path", mount.Source, err)
			normalizedSource = mount.Source
		}
		if normalizedSource == normalizedPVCPath || mount.Source == pvcHostPath {
			log.Printf("Removing PVC mount (now root): %s -> %s", mount.Source, mount.Destination)
			continue
		}
		filteredMounts = append(filteredMounts, mount)
	}
	spec.Mounts = filteredMounts

	// Write back the modified config (0600 to protect sensitive info)
	modifiedData, err := json.MarshalIndent(&spec, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal modified config: %w", err)
	}

	if err := os.WriteFile(configPath, modifiedData, 0600); err != nil {
		return fmt.Errorf("failed to write modified config.json: %w", err)
	}

	log.Printf("Successfully modified config.json for SGS boot volume with overlayfs")
	return nil
}

// findSGSOSVolumeMountSource searches mounts for the SGS OS volume by its known destination path.
// When CLI attaches an os-volume, it is guaranteed to be mounted at /sgs-os-volume.
// We find this mount and return its source path (the PVC host path).
func findSGSOSVolumeMountSource(spec *specs.Spec) string {
	for _, mount := range spec.Mounts {
		if mount.Destination == sgsOSVolumeMountPath {
			log.Printf("Found SGS OS volume mount: %s -> %s", mount.Source, mount.Destination)
			return mount.Source
		}
	}
	return ""
}

// checkKernelVersion verifies the kernel supports nested overlayfs (5.11+).
// containerd's snapshotter typically uses overlayfs to prepare rootfs from image layers.
// We use that as lowerdir, which requires kernel 5.11+ for nested overlay support.
func checkKernelVersion() error {
	var uname unix.Utsname
	if err := unix.Uname(&uname); err != nil {
		return fmt.Errorf("failed to get kernel version: %w", err)
	}

	// Convert release bytes to string (e.g., "5.15.0-generic")
	release := unix.ByteSliceToString(uname.Release[:])

	var major, minor int
	if _, err := fmt.Sscanf(release, "%d.%d", &major, &minor); err != nil {
		return fmt.Errorf("failed to parse kernel version '%s': %w", release, err)
	}

	if major < minKernelMajor || (major == minKernelMajor && minor < minKernelMinor) {
		return fmt.Errorf("kernel %d.%d is too old for nested overlayfs; "+
			"SGS requires kernel %d.%d+ (found: %s)",
			major, minor, minKernelMajor, minKernelMinor, release)
	}

	log.Printf("Kernel version %s supports nested overlayfs", release)
	return nil
}

// isMountpoint checks if the given path is a mount point by comparing
// the device ID of the path and its parent directory.
func isMountpoint(path string) bool {
	var pathStat, parentStat unix.Stat_t

	if err := unix.Stat(path, &pathStat); err != nil {
		return false
	}

	parent := filepath.Dir(path)
	if err := unix.Stat(parent, &parentStat); err != nil {
		return false
	}

	// If device IDs differ, path is a mount point
	return pathStat.Dev != parentStat.Dev
}

// clearDirectory removes all contents of a directory but keeps the directory itself.
// This is required for overlayfs workdir which must be empty before mounting.
func clearDirectory(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist, nothing to clear
		}
		return fmt.Errorf("failed to read directory %s: %w", path, err)
	}

	for _, entry := range entries {
		entryPath := filepath.Join(path, entry.Name())
		if err := os.RemoveAll(entryPath); err != nil {
			return fmt.Errorf("failed to remove %s: %w", entryPath, err)
		}
	}

	return nil
}

// setupOverlayfs creates an overlayfs mount with the container image as lowerdir
// and the PVC as upperdir. Returns the path to the merged directory.
func setupOverlayfs(lowerdir, pvcPath string) (string, error) {
	upperDir := filepath.Join(pvcPath, overlayUpperDir)
	workDir := filepath.Join(pvcPath, overlayWorkDir)
	mergedDir := filepath.Join(pvcPath, overlayMergedDir)

	// 1. Check kernel version (nested overlay requires 5.11+)
	if err := checkKernelVersion(); err != nil {
		return "", err
	}

	// 2. Create directories
	for _, dir := range []string{upperDir, workDir, mergedDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	// 3. Clear work directory (overlayfs requires empty workdir)
	if err := clearDirectory(workDir); err != nil {
		return "", fmt.Errorf("failed to clear workdir: %w", err)
	}

	// 4. Unmount if already mounted (handles container restart)
	if isMountpoint(mergedDir) {
		log.Printf("Detected existing mount at %s, unmounting", mergedDir)
		if err := unix.Unmount(mergedDir, 0); err != nil {
			log.Printf("Warning: failed to unmount existing overlay: %v; trying lazy unmount", err)
			// Try lazy unmount as fallback
			if err := unix.Unmount(mergedDir, unix.MNT_DETACH); err != nil {
				return "", fmt.Errorf("failed to unmount existing overlay at %s: %w", mergedDir, err)
			}
		}
	}

	// 5. Mount overlayfs
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerdir, upperDir, workDir)

	if err := unix.Mount("overlay", mergedDir, "overlay", 0, opts); err != nil {
		// Provide helpful error for nested overlay failure
		if errors.Is(err, unix.EINVAL) {
			return "", fmt.Errorf("overlayfs mount failed (EINVAL): this may indicate "+
				"nested overlayfs is not supported on this kernel; "+
				"SGS requires kernel 5.11+ for nested overlay: %w", err)
		}
		return "", fmt.Errorf("failed to mount overlayfs: %w", err)
	}

	log.Printf("Mounted overlayfs: lowerdir=%s, upperdir=%s, merged=%s",
		lowerdir, upperDir, mergedDir)

	return mergedDir, nil
}

