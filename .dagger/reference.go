package main

import "go.podman.io/image/v5/docker/reference"

func getRegistryFromImageRepo(imageRepo string) string {
	named, err := reference.ParseNormalizedNamed(imageRepo)
	if err != nil {
		return ""
	}
	return reference.Domain(named)
}
