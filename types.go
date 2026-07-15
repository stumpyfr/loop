package main

const (
	defaultRegistry     = "ghcr.io"
	defaultArtifactType = "application/vnd.arkham.loop.package.v1+yaml"
	defaultLayerType    = "application/vnd.arkham.loop.package.config.v1+yaml"
	sourceAnnotation    = "org.opencontainers.image.source"
	fixedCreatedTime    = "1970-01-01T00:00:00Z"
)

type options struct {
	command       string
	filename      string
	output        string
	targetRef     string
	registry      string
	artifactType  string
	layerType     string
	helpTopic     string
	agentsFile    string
	renderNoColor bool
	renderDetails bool
	source        string
}

type pushResult struct {
	digest  string
	skipped bool
}

type pullResult struct {
	ref            string
	manifestDigest string
	cachePath      string
	updated        bool
}
