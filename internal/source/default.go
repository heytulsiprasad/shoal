package source

// NewTorlinkSources returns the full source set ported from torlink.
func NewTorlinkSources() []Source {
	return []Source{
		NewFitGirl(),
		NewYTS(),
		NewPirateBayMovies(),
		New1337xMovies(),
		NewEZTV(),
		NewSolidTorrents(),
		NewPirateBayTV(),
		New1337xTV(),
		NewNyaa(),
		NewSubsPlease(),
	}
}

// NewDefault returns shoal's default multi-source catalogue.
func NewDefault() *MultiSource {
	sources := []Source{NewArchive(), NewCurated()}
	sources = append(sources, NewTorlinkSources()...)
	return NewMulti(sources...)
}
