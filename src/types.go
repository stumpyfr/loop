package main

const (
	defaultRegistry = "ghcr.io"

	loopArtifactType     = "application/vnd.agentloops.loop.v1"
	loopLayerType        = "application/vnd.agentloops.loop.content.v1+yaml"
	loopCollectionType   = "application/vnd.agentloops.loop.collection.v1"
	loopConfigType       = "application/vnd.agentloops.loop.config.v1+json"
	skillArtifactType    = "application/vnd.agentskills.skill.v1"
	skillLayerType       = "application/vnd.agentskills.skill.content.v1.tar+gzip"
	skillCollectionType  = "application/vnd.agentskills.collection.v1"
	skillConfigType      = "application/vnd.agentskills.skill.config.v1+json"
	sourceAnnotation     = "org.opencontainers.image.source"
	fixedCreatedTime     = "1970-01-01T00:00:00Z"
	defaultAgentsDir     = ".agents"
	agentkitManifestName = "agentkit.json"
	agentkitLockName     = "agentkit.lock.json"
	loopNameAnnotation   = "io.agentloops.loop.name"
	loopRefAnnotation    = "io.agentloops.loop.ref"
	skillNameAnnotation  = "io.agentskills.skill.name"
	skillRefAnnotation   = "io.agentskills.skill.ref"
	collectionNameSuffix = ".json"
	defaultArtifactType  = loopArtifactType
	defaultLayerType     = loopLayerType
)

type options struct {
	command        string
	domain         string
	action         string
	resource       string
	filename       string
	output         string
	targetRef      string
	registry       string
	artifactType   string
	layerType      string
	collectionType string
	helpTopic      string
	agentsFile     string
	agentsDir      string
	renderNoColor  bool
	renderDetails  bool
	source         string
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
