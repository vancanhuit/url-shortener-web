package main

import "runtime/debug"

func Version() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	return bi.Main.Version
}
