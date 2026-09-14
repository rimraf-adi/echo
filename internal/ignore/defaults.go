package ignore

// DefaultPatterns contains patterns that are always ignored in Echo workspaces
var DefaultPatterns = []string{
	".echo",
	".echo/**",
	".git",
	".git/**",
}
