// Package ordered declares enough names two rules match that an order made
// by chance would show.
package ordered

var cfgA, cfgB, cfgC = 1, 2, 3 // want `"cfgA" is forbidden by pattern "cfg"` `"cfgA" is forbidden: again` `"cfgB" is forbidden by pattern "cfg"` `"cfgB" is forbidden: again` `"cfgC" is forbidden by pattern "cfg"` `"cfgC" is forbidden: again`

func cfgD(cfgE, cfgF int) (cfgG int) { // want `"cfgD" is forbidden by pattern "cfg"` `"cfgD" is forbidden: again` `"cfgE" is forbidden by pattern "cfg"` `"cfgE" is forbidden: again` `"cfgF" is forbidden by pattern "cfg"` `"cfgF" is forbidden: again` `"cfgG" is forbidden by pattern "cfg"` `"cfgG" is forbidden: again`
	cfgH, cfgI := cfgE, cfgF // want `"cfgH" is forbidden by pattern "cfg"` `"cfgH" is forbidden: again` `"cfgI" is forbidden by pattern "cfg"` `"cfgI" is forbidden: again`
	return cfgH + cfgI
}

type cfgJ struct { // want `"cfgJ" is forbidden by pattern "cfg"` `"cfgJ" is forbidden: again`
	cfgK, cfgL int // want `"cfgK" is forbidden by pattern "cfg"` `"cfgK" is forbidden: again` `"cfgL" is forbidden by pattern "cfg"` `"cfgL" is forbidden: again`
}
