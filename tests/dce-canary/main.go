// dce-canary is a positive control for "make verify-dce". It calls
// reflect.Value.MethodByName with a non-constant argument, which makes
// the linker disable dead code elimination of exported methods, so
// "verify-dce" must detect it. If it does not, the check is broken.
package main

import (
	"os"
	"reflect"
)

type t struct{}

func (t) Foo() {}

func main() {
	reflect.ValueOf(t{}).MethodByName(os.Args[0])
}
