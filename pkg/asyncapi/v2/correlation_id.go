package asyncapiv2

// CorrelationID is a representation of the corresponding asyncapi object filled
// from an asyncapi specification that will be used to generate code.
// Source: https://www.asyncapi.com/docs/reference/specification/v2.6.0#correlationIdObject
type CorrelationID struct {
	// --- AsyncAPI fields -----------------------------------------------------

	Description string `json:"description"`
	Location    string `json:"location"`

	// --- Non AsyncAPI fields -------------------------------------------------
}
