// Package uses reads names the rules match without declaring them, but for
// the one variable whose uses it reads.
package uses

import "external"

var _ = external.Cfg

type embedding struct {
	external.CfgEmbedded
	*external.Client
}

func run() int {
	cfg := load() // want `"cfg" is forbidden by pattern "cfg" for local-var$`
	use(cfg)
	cfg = load()
	cfg, err := reload()
	use(err)
	var _ = cfg
	use(external.Cfg())
	use(external.CfgValue)
	client := &external.Client{CfgField: cfg}
	client.CfgMethod()
	method := (*external.Client).CfgMethod
	use(method)
	use(embedding{}.CfgEmbedded)
	return client.CfgField
}

func load() int { return 0 }

func reload() (int, error) { return 0, nil }

func use(any) {}
