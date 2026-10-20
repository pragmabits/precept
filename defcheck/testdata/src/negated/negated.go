// Package negated declares names a rule matches in every kind but the one it
// leaves out.
package negated

type settings struct {
	cfgField int // want `"cfgField" is forbidden by pattern "cfg" for field$`
}

func cfgLoad(cfgParameter int) settings { // want `"cfgLoad" is forbidden by pattern "cfg" for function$` `"cfgParameter" is forbidden by pattern "cfg" for parameter$`
	cfgLocal := cfgParameter
	return settings{cfgField: cfgLocal}
}
