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
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	// realRuncPath is the path to the actual runc binary
	realRuncPath = "/usr/bin/runc"

	// annotationOSVolume is the annotation that triggers rootfs replacement.
	// The value is the PVC name, which we use to find the mount source.
	annotationOSVolume = "sgs.snucse.org/os-volume"
)

func main() {
	// Set up logging to a file for debugging
	logFile, err := os.OpenFile("/var/log/sgs-runc-wrapper.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
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
	if isCreate && bundlePath != "" {
		if err := maybeModifyRootfs(bundlePath); err != nil {
			log.Printf("Warning: continuing without SGS rootfs modification; this is expected for non-SGS containers but may indicate a problem (e.g., failed to read or parse config.json): %v", err)
			// Continue anyway so that non-SGS containers still run; runc will surface any fatal errors
		}
	}

	// Execute the real runc
	log.Printf("Executing real runc: %s %v", realRuncPath, args)
	execErr := syscall.Exec(realRuncPath, append([]string{"runc"}, args...), os.Environ())
	if execErr != nil {
		log.Printf("Failed to exec runc: %v", execErr)
		// Fallback to exec.Command if syscall.Exec fails
		cmd := exec.Command(realRuncPath, args...)
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
		log.Printf("No annotations found, skipping modification")
		return nil
	}

	bootVolumeClaim, hasBootVolume := spec.Annotations[annotationOSVolume]
	if !hasBootVolume {
		log.Printf("No os-volume annotation, skipping modification")
		return nil
	}

	log.Printf("Found os-volume annotation: %s", bootVolumeClaim)

	// Find the PVC host path by searching mounts for one containing the PVC name.
	// Kubelet mounts PVCs at paths like:
	// /var/lib/kubelet/pods/<uid>/volumes/kubernetes.io~csi/<pvc-name>/mount
	pvcHostPath := findPVCMountSource(&spec, bootVolumeClaim)

	if pvcHostPath == "" {
		return fmt.Errorf("boot volume annotation present but could not find PVC mount for '%s'", bootVolumeClaim)
	}

	log.Printf("PVC host path: %s", pvcHostPath)

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
	filteredMounts := make([]specs.Mount, 0, len(spec.Mounts))
	for _, mount := range spec.Mounts {
		if mount.Source == pvcHostPath {
			log.Printf("Removing PVC mount (now root): %s -> %s", mount.Source, mount.Destination)
			continue
		}
		filteredMounts = append(filteredMounts, mount)
	}
	spec.Mounts = filteredMounts

	// Write back the modified config
	modifiedData, err := json.MarshalIndent(&spec, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal modified config: %w", err)
	}

	if err := os.WriteFile(configPath, modifiedData, 0644); err != nil {
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

