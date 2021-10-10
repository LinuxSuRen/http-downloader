package compress

// Compress is a common compress interface
type Compress interface {
	ExtractFiles(sourceFile, targetName string) error
}
