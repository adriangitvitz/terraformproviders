package main

import (
	"kindcluster/clusterkind"

	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: clusterkind.Provider,
	})
}
