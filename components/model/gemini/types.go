package gemini

import (
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
	genai "google.golang.org/genai"
)

// Config contains the configuration options for the Gemini model
type Config struct {
	// Client is the Gemini API client instance
	// Required for making API calls to Gemini
	Client *genai.Client

	// Model specifies which Gemini model to use
	// Examples: "gemini-pro", "gemini-pro-vision", "gemini-1.5-flash"
	Model string

	// Publisher specifies the model publisher (e.g., "google", "anthropic")
	// Optional. Default: "google"
	Publisher string

	// Project specifies the Google Cloud project ID
	// Optional. Used for constructing full Vertex AI resource names
	Project string

	// Location specifies the Google Cloud location/region
	// Optional. Used for constructing full Vertex AI resource names
	Location string

	// CredentialsPath specifies the path to Google application credentials JSON file
	// Required for authentication with Google Cloud services
	CredentialsPath string

	// GenerateContentConfig contains all Gemini generation parameters
	// Optional. Provides full control over Gemini's generation behavior
	// This directly uses genai.GenerateContentConfig for maximum compatibility
	GenerateContentConfig *genai.GenerateContentConfig

	// ResponseSchema defines the structure for JSON responses
	// Optional. Used when you want structured output in JSON format
	ResponseSchema *openapi3.Schema
}

// panicErr represents a panic that occurred during execution
// Used for capturing and propagating panic information in goroutines
type panicErr struct {
	info  any
	stack []byte
}

// Error implements the error interface for panicErr
func (p *panicErr) Error() string {
	return fmt.Sprintf("panic error: %v, \nstack: %s", p.info, string(p.stack))
}

// newPanicErr creates a new panicErr with the given panic info and stack trace
func newPanicErr(info any, stack []byte) error {
	return &panicErr{
		info:  info,
		stack: stack,
	}
}
