// Package language declares names in the forms of the language that declare
// more than one name, or one name more than once.
package language

func shortDeclaration() {
	cfgFirst, err := pair() // want `"cfgFirst" is forbidden by pattern "cfg" for local-var$`
	cfgFirst, cfgSecond := pair() // want `"cfgSecond" is forbidden by pattern "cfg" for local-var$`
	use(cfgFirst, cfgSecond, err)
}

func typeSwitch(value any) {
	switch cfgSwitch := value.(type) { // want `"cfgSwitch" is forbidden by pattern "cfg" for local-var$`
	case int:
		use(cfgSwitch)
	case string:
		use(cfgSwitch)
	default:
		use(cfgSwitch)
	}
}

func shadowing() {
	cfgShadow := 1 // want `"cfgShadow" is forbidden by pattern "cfg" for local-var$`
	{
		cfgShadow := 2 // want `"cfgShadow" is forbidden by pattern "cfg" for local-var$`
		use(cfgShadow)
	}
	use(cfgShadow)
}

func closures() {
	cfgClosure := func(cfgInner int) (cfgOuter int) { // want `"cfgClosure" is forbidden by pattern "cfg" for local-var$` `"cfgInner" is forbidden by pattern "cfg" for parameter$` `"cfgOuter" is forbidden by pattern "cfg" for result$`
		cfgNested := cfgInner // want `"cfgNested" is forbidden by pattern "cfg" for local-var$`
		return cfgNested
	}
	use(cfgClosure)
}

var cfgOne, cfgTwo = 1, 2 // want `"cfgOne" is forbidden by pattern "cfg" for package-var$` `"cfgTwo" is forbidden by pattern "cfg" for package-var$`

var (
	cfgGrouped int // want `"cfgGrouped" is forbidden by pattern "cfg" for package-var$`
	cfgOther   int // want `"cfgOther" is forbidden by pattern "cfg" for package-var$`
)

const cfgThree, cfgFour = 3, 4 // want `"cfgThree" is forbidden by pattern "cfg" for constant$` `"cfgFour" is forbidden by pattern "cfg" for constant$`

type value struct {
	cfgLeft, cfgRight int // want `"cfgLeft" is forbidden by pattern "cfg" for field$` `"cfgRight" is forbidden by pattern "cfg" for field$`
}

func (cfgPointer *value) pointer() {} // want `"cfgPointer" is forbidden by pattern "cfg" for receiver$`

func (cfgValue value) plain() {} // want `"cfgValue" is forbidden by pattern "cfg" for receiver$`

type mapping[cfgKey comparable, cfgElement any] struct{} // want `"cfgKey" is forbidden by pattern "cfg" for type-parameter$` `"cfgElement" is forbidden by pattern "cfg" for type-parameter$`

func (m mapping[cfgK, cfgE]) get() {} // want `"cfgK" is forbidden by pattern "cfg" for type-parameter$` `"cfgE" is forbidden by pattern "cfg" for type-parameter$`

type callback func(cfgArgument int) (cfgReturned int) // want `"cfgArgument" is forbidden by pattern "cfg" for parameter$` `"cfgReturned" is forbidden by pattern "cfg" for result$`

func pair() (int, error) { return 0, nil }

func use(...any) {}
