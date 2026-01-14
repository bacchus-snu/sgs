// Package main implements the SGS OCI runtime wrapper binary `sgs-runc-wrapper`.
//
// This program wraps runc (or any OCI runtime) and intercepts the "create"
// command to modify the OCI spec's Root.Path. This provides true rootfs
// replacement for "Stateful Containers".
//
// How it works:
//  1. containerd calls this wrapper instead of runc directly
//  2. Wrapper checks if the container has SGS boot volume annotation
//  3. If present, modifies config.json Root.Path to the PVC host path
//  4. Calls the real runc with the modified config
//
// Installation:
//  1. Build: go build -o sgs-runc-wrapper ./cmd/sgs-runc-wrapper
//  2. Install: cp sgs-runc-wrapper /usr/local/bin/
//  3. Configure containerd (see below)
//
// Containerd configuration (/etc/containerd/config.toml):
//
//	[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.sgs]
//	  runtime_type = "io.containerd.runc.v2"
//	  [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.sgs.options]
//	    BinaryName = "/usr/local/bin/sgs-runc-wrapper"
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
)

// errNoSGSAnnotation is returned when a container doesn't have SGS annotation (expected for non-SGS containers)
var errNoSGSAnnotation = errors.New("no SGS os-volume annotation found")

const (
	// defaultRuncPath is the default path to the actual runc binary
	defaultRuncPath = "/usr/bin/runc"

	// envRuncPath is the environment variable to override runc path
	envRuncPath = "SGS_RUNC_PATH"

	// annotationOSVolume is the annotation that triggers rootfs replacement.
	// The value is the PVC name, which we use to find the mount source.
	annotationOSVolume = "sgs.snucse.org/os-volume"

	// kubeletVolumesPrefix is the expected prefix for kubelet-managed PVC paths
	kubeletVolumesPrefix = "/var/lib/kubelet/pods/"
)

// getRuncPath returns the path to the real runc binary.
// It checks SGS_RUNC_PATH env var first, then tries exec.LookPath, finally falls back to default.
func getRuncPath() string {
	if path := os.Getenv(envRuncPath); path != "" {
		log.Printf("Using runc path from %s: %s", envRuncPath, path)
		return path
	}
	if path, err := exec.LookPath("runc"); err == nil {
		log.Printf("Found runc via PATH: %s", path)
		return path
	}
	log.Printf("Using default runc path: %s", defaultRuncPath)
	return defaultRuncPath
}

func main() {
	// Set up logging to a file for debugging (0600 to protect sensitive info)
	logFile, err := os.OpenFile("/var/log/sgs-runc-wrapper.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err == nil {
		log.SetOutput(logFile)
		defer func() {
			if err := logFile.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "sgs-runc-wrapper: failed to close log file: %v\n", err)
			}
		}()
	} else {
		fmt.Fprintf(os.Stderr, "sgs-runc-wrapper: failed to open log file /var/log/sgs-runc-wrapper.log: %v\n", err)
	}

	args := os.Args[1:]

	log.Printf("sgs-runc-wrapper called with args: %v", args)

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
			// Check if this is an expected "no SGS annotation" case or an actual error
			if errors.Is(err, errNoSGSAnnotation) {
				log.Printf("Info: %v", err)
			} else {
				log.Printf("Error: SGS rootfs modification failed: %v", err)
				// For SGS containers with actual errors, fail fast rather than proceeding with wrong config
				fmt.Fprintf(os.Stderr, "sgs-runc-wrapper: %v\n", err)
				os.Exit(1)
			}
		}
	}

	// Execute the real runc
	runcPath := getRuncPath()
	log.Printf("Executing real runc: %s %v", runcPath, args)
	execErr := syscall.Exec(runcPath, append([]string{"runc"}, args...), os.Environ())
	if execErr != nil {
		log.Printf("Failed to exec runc: %v", execErr)
		// Fallback to exec.Command if syscall.Exec fails
		cmd := exec.Command(runcPath, args...)
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

// maybeModifyRootfs checks the OCI spec for SGS annotations and modifies Root.Path if needed.
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

	// Check for boot volume annotation
	if spec.Annotations == nil {
		return errNoSGSAnnotation
	}

	bootVolumeClaim, hasBootVolume := spec.Annotations[annotationOSVolume]
	if !hasBootVolume {
		return errNoSGSAnnotation
	}

	log.Printf("Found os-volume annotation: %s", bootVolumeClaim)

	// Find the PVC host path by searching mounts for one containing the PVC name.
	// Kubelet mounts PVCs at paths like:
	// /var/lib/kubelet/pods/<uid>/volumes/kubernetes.io~csi/<pvc-name>/mount
	pvcHostPath := findPVCMountSource(&spec, bootVolumeClaim)

	if pvcHostPath == "" {
		return fmt.Errorf("boot volume annotation present but could not find PVC mount for '%s'. "+
			"Ensure the PVC exists, is mounted to the pod, and the volume mount is specified in the pod spec", bootVolumeClaim)
	}

	log.Printf("PVC host path: %s", pvcHostPath)

	// Validate that the PVC path is within expected kubelet directory to prevent
	// malicious annotations pointing to arbitrary host directories
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

	// Store the original root path for debugging
	originalRoot := spec.Root.Path
	log.Printf("Original root path: %s", originalRoot)

	// CRITICAL: Modify the root path to point to the PVC
	// This is the key difference from NRI - we're changing Root.Path directly
	spec.Root.Path = pvcHostPath
	spec.Root.Readonly = false

	log.Printf("New root path: %s", spec.Root.Path)

	// Add annotations to track the change
	spec.Annotations["sgs.snucse.org/original-root"] = originalRoot
	spec.Annotations["sgs.snucse.org/boot-volume-active"] = "true"

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

	log.Printf("Successfully modified config.json for SGS boot volume")
	return nil
}

// findPVCMountSource searches mounts for one that corresponds to the given PVC.
// Kubelet typically mounts PVCs at paths like:
//   /var/lib/kubelet/pods/<pod-uid>/volumes/kubernetes.io~csi/<pvc-name>/mount
//   /var/lib/kubelet/pods/<pod-uid>/volumes/kubernetes.io~<type>/<pvc-name>
//
// We search for the PVC name as a path segment in the mount source path.
func findPVCMountSource(spec *specs.Spec, pvcName string) string {
	for _, mount := range spec.Mounts {
		// Normalize and split the source path into segments, then look for pvcName as a full segment.
		cleanSource := filepath.Clean(mount.Source)
		segments := strings.Split(cleanSource, string(os.PathSeparator))
		for _, segment := range segments {
			if segment == pvcName {
				log.Printf("Found PVC mount by name '%s': %s -> %s", pvcName, mount.Source, mount.Destination)
				return mount.Source
			}
		}
	}

	// Fallback: if PVC name not found directly, log all mounts for debugging
	log.Printf("Could not find mount with PVC name '%s' in source path", pvcName)
	for i, mount := range spec.Mounts {
		log.Printf("  Mount[%d]: %s -> %s", i, mount.Source, mount.Destination)
	}

	return ""
}

