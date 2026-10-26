// Package marmot embeds files from the repository root that the binary ships.
package marmot

import _ "embed"

// Skill is the agent skill for this version of Marmot, installed by `marmot skills install`.
//
//go:embed SKILL.md
var Skill []byte
