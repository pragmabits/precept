// Package external stands for a dependency of the analyzed packages: what it
// declares is declared outside them, and no pass over them reports it.
package external

func Cfg() int { return 0 }

var CfgValue = 1

type Client struct {
	CfgField int
}

func (c *Client) CfgMethod() {}

type CfgEmbedded struct{}
