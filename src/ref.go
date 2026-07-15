package main

import (
	"fmt"
	"strings"
)

func normalizeTargetRef(ref string) (string, string, error) {
	if strings.TrimSpace(ref) != ref || ref == "" {
		return "", "", fmt.Errorf("target reference must not be empty or contain surrounding whitespace: %q", ref)
	}
	if strings.ContainsAny(ref, " \t\r\n") {
		return "", "", fmt.Errorf("target reference contains whitespace: %q", ref)
	}

	firstSlash := strings.Index(ref, "/")
	if firstSlash <= 0 || firstSlash == len(ref)-1 {
		return "", "", fmt.Errorf("target reference must look like registry/namespace/package_name:tag, got %q", ref)
	}

	lastSlash := strings.LastIndex(ref, "/")
	if lastSlash == firstSlash {
		return "", "", fmt.Errorf("target reference must include a namespace and package name, got %q", ref)
	}
	lastColon := strings.LastIndex(ref, ":")
	if lastColon <= lastSlash || lastColon == len(ref)-1 {
		return "", "", fmt.Errorf("target reference must include an explicit tag, got %q", ref)
	}

	name := ref[:lastColon]
	tag := ref[lastColon+1:]
	if strings.Contains(tag, "/") {
		return "", "", fmt.Errorf("target reference tag is invalid: %q", ref)
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" {
			return "", "", fmt.Errorf("target reference contains empty path components: %q", ref)
		}
	}
	for _, part := range strings.Split(name[firstSlash+1:], "/") {
		if strings.Contains(part, ":") {
			return "", "", fmt.Errorf("target reference path contains invalid ':' character: %q", ref)
		}
	}

	return strings.ToLower(name) + ":" + tag, strings.ToLower(ref[:firstSlash]), nil
}

func targetName(ref string) string {
	lastColon := strings.LastIndex(ref, ":")
	if lastColon == -1 {
		return ref
	}
	return ref[:lastColon]
}

func targetRepository(ref string) string {
	name := targetName(ref)
	firstSlash := strings.Index(name, "/")
	if firstSlash == -1 || firstSlash == len(name)-1 {
		return name
	}
	return name[firstSlash+1:]
}

func targetTag(ref string) string {
	lastColon := strings.LastIndex(ref, ":")
	if lastColon == -1 || lastColon == len(ref)-1 {
		return ""
	}
	return ref[lastColon+1:]
}
