/*
Copyright © 2024, Altinn
*/
package main

import (
	"github.com/altinn/altinn-platform/daisctl/cmd"
	"github.com/altinn/altinn-platform/daisctl/internal/version"
)

func main() {
	cmd.BuildInfo = version.Get()
	cmd.Execute()
}
