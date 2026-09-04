//go:build !unix

package preview

import "os"

// ownerGroup has no portable non-unix implementation; numeric-style
// placeholders keep the inspector usable.
func ownerGroup(string, os.FileInfo) (string, string) { return "?", "?" }
