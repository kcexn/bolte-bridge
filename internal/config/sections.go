package config

// DefaultSections is the set of configuration sections the bridge builds by
// default; main passes it to Load. To add configuration, write a SectionFunc
// (see storeSection for the shape) and append it here — nothing else changes.
var DefaultSections = []SectionFunc{
	storeSection,
	emailSection,
	matrixSection,
}
