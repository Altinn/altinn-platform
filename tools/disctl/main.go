/*
Copyright © 2024, Altinn
*/
package main

import (
	"github.com/altinn/altinn-platform/disctl/cmd"
	"github.com/altinn/altinn-platform/disctl/internal/version"
)

func main() {
	cmd.BuildInfo = version.Get()
	cmd.Execute()
}
