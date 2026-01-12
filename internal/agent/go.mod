module github.com/nathfavour/vibeauracle/agent

go 1.25.5

require (
	github.com/nathfavour/vibeauracle/prompt v0.0.0
	github.com/nathfavour/vibeauracle/tooling v0.0.0
	github.com/nathfavour/vibeauracle/sys v0.0.0
)

replace (
	github.com/nathfavour/vibeauracle/prompt => ../prompt
	github.com/nathfavour/vibeauracle/tooling => ../tooling
	github.com/nathfavour/vibeauracle/sys => ../sys
)
