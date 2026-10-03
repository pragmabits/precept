package testfuncs

import "testing"

func TestCfg(t *testing.T) {}

func Test_cfg(t *testing.T) {}

func Test(t *testing.T) {}

func BenchmarkCfg(b *testing.B) {}

func FuzzCfg(f *testing.F) {}

func ExampleCfg() {}

func Example_cfg() {}

func TestMain(m *testing.M) {}

// Testcfg is not run: a lower-case letter follows the prefix.
func Testcfg(t *testing.T) {} // want `"Testcfg" is forbidden by pattern`

// ExampleCfgArgument is not run: it has a parameter.
func ExampleCfgArgument(count int) {} // want `"ExampleCfgArgument" is forbidden by pattern`

// suite declares a method named as a test, which go test does not run.
type suite struct{}

func (suite) TestCfg(t *testing.T) {} // want `"TestCfg" is forbidden by pattern`
