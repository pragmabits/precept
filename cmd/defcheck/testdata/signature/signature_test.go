package signature

import "testing"

func TestCfg(t *testing.T) {}

func TestCfgArgument(count int) {}

func TestCfgResult(t *testing.T) bool { return true }

func BenchmarkCfgOther(t *testing.T) {}

func FuzzCfgGeneric[T any](f *testing.F) {}
