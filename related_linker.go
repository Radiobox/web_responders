package web_responders

// A SelfLinker returns links to values related to itself.
type RelatedLinker interface {

	// Links should return a map of rel:link key:value pairs which
	// will be added to the Link header.
	RelatedLinks() map[string]string
}
