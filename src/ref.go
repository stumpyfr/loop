package main

import (
	"fmt"
	"strings"

	"github.com/opencontainers/go-digest"
	"oras.land/oras-go/v2/registry"
)

func normalizeTargetRef(ref string) (string, string, error) {
	if strings.TrimSpace(ref) != ref || ref == "" {
		return "", "", fmt.Errorf("target reference must not be empty or contain surrounding whitespace: %q", ref)
	}
	if strings.ContainsAny(ref, " \t\r\n") {
		return "", "", fmt.Errorf("target reference contains whitespace: %q", ref)
	}

	name, tag, digestRef, err := splitOCIReference(ref)
	if err != nil {
		return "", "", err
	}
	if tag == "" && digestRef == "" {
		return "", "", fmt.Errorf("target reference must include an explicit tag or digest, got %q", ref)
	}

	normalized := strings.ToLower(name)
	if tag != "" {
		if err := (registry.Reference{Reference: tag}).ValidateReferenceAsTag(); err != nil {
			return "", "", fmt.Errorf("target reference tag is invalid: %q", ref)
		}
		normalized += ":" + tag
	}
	if digestRef != "" {
		parsed, err := digest.Parse(digestRef)
		if err != nil {
			return "", "", fmt.Errorf("target reference digest is invalid: %q", ref)
		}
		normalized += "@" + parsed.String()
	}
	parsed, err := registry.ParseReference(normalized)
	if err != nil {
		return "", "", fmt.Errorf("target reference is not a valid OCI reference: %q: %w", ref, err)
	}
	if parsed.Reference == "" {
		return "", "", fmt.Errorf("target reference must include an explicit tag or digest, got %q", ref)
	}
	if !strings.Contains(parsed.Repository, "/") {
		return "", "", fmt.Errorf("target reference must include a namespace and package name, got %q", ref)
	}
	return normalized, strings.ToLower(parsed.Registry), nil
}

func splitOCIReference(ref string) (name string, tag string, digestRef string, err error) {
	nameTag := ref
	if at := strings.LastIndex(ref, "@"); at >= 0 {
		if at == len(ref)-1 {
			return "", "", "", fmt.Errorf("target reference digest is empty: %q", ref)
		}
		nameTag = ref[:at]
		digestRef = ref[at+1:]
	}
	lastSlash := strings.LastIndex(nameTag, "/")
	lastColon := strings.LastIndex(nameTag, ":")
	name = nameTag
	if lastColon > lastSlash {
		if lastColon == len(nameTag)-1 {
			return "", "", "", fmt.Errorf("target reference tag is empty: %q", ref)
		}
		name = nameTag[:lastColon]
		tag = nameTag[lastColon+1:]
	}
	return name, tag, digestRef, nil
}

func targetName(ref string) string {
	nameTag := refWithoutDigest(ref)
	lastSlash := strings.LastIndex(nameTag, "/")
	lastColon := strings.LastIndex(nameTag, ":")
	if lastColon > lastSlash {
		return nameTag[:lastColon]
	}
	return nameTag
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
	nameTag := refWithoutDigest(ref)
	lastSlash := strings.LastIndex(nameTag, "/")
	lastColon := strings.LastIndex(nameTag, ":")
	if lastColon <= lastSlash || lastColon == len(nameTag)-1 {
		return ""
	}
	return nameTag[lastColon+1:]
}

func targetDigest(ref string) string {
	if at := strings.LastIndex(ref, "@"); at >= 0 && at < len(ref)-1 {
		return ref[at+1:]
	}
	return ""
}

func refWithoutDigest(ref string) string {
	if at := strings.LastIndex(ref, "@"); at >= 0 {
		return ref[:at]
	}
	return ref
}
