package eino

const (
	metadataKeyOfEnableStreaming = "_a2a_eino_adk_enable_streaming"
	metadataKeyOfInterrupted     = "_a2a_eino_adk_interrupted"
)

func setEnableStreaming(metadata map[string]any) {
	metadata[metadataKeyOfEnableStreaming] = true
}

func getEnableStreaming(metadata map[string]any) bool {
	b, ok := metadata[metadataKeyOfEnableStreaming]
	return ok && b == true
}

func setInterrupted(metadata map[string]any) {
	metadata[metadataKeyOfInterrupted] = true
}

func getInterrupted(metadata map[string]any) bool {
	b, ok := metadata[metadataKeyOfInterrupted]
	return ok && b == true
}
