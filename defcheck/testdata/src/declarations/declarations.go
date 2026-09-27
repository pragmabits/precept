// Package declarations declares a name of every kind a rule applies to.
package declarations

var cfgPackage = 1 // want `^declaration name "cfgPackage" is forbidden by pattern "cfg" for package-var$`

const cfgConstant = 2 // want `^declaration name "cfgConstant" is forbidden by pattern "cfg" for constant$`

type cfgType struct { // want `"cfgType" is forbidden by pattern "cfg" for type$`
	cfgField int // want `"cfgField" is forbidden by pattern "cfg" for field$`
}

type cfgAlias = cfgType // want `"cfgAlias" is forbidden by pattern "cfg" for type$`

func cfgFunction() {} // want `"cfgFunction" is forbidden by pattern "cfg" for function$`

func (cfgReceiver *cfgType) cfgMethod() {} // want `"cfgReceiver" is forbidden by pattern "cfg" for receiver$` `"cfgMethod" is forbidden by pattern "cfg" for method$`

type store interface {
	cfgInterfaceMethod() // want `"cfgInterfaceMethod" is forbidden by pattern "cfg" for method$`
}

func signature(cfgParameter int) (cfgResult int) { // want `"cfgParameter" is forbidden by pattern "cfg" for parameter$` `"cfgResult" is forbidden by pattern "cfg" for result$`
	cfgLocal := cfgParameter // want `"cfgLocal" is forbidden by pattern "cfg" for local-var$`
	return cfgLocal
}

func generic[cfgFunctionParameter any]() {} // want `"cfgFunctionParameter" is forbidden by pattern "cfg" for type-parameter$`

type list[cfgTypeParameter any] struct{} // want `"cfgTypeParameter" is forbidden by pattern "cfg" for type-parameter$`

func (l *list[cfgReceiverParameter]) push() {} // want `"cfgReceiverParameter" is forbidden by pattern "cfg" for type-parameter$`
