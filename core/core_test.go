package core

import (
	"reflect"
	"testing"
)

type mod struct{ D Descriptor }

func (m mod) Describe() Descriptor { return m.D }
func TestDependencyOrderAndValidation(t *testing.T) {
	a := mod{Descriptor{ID: "auth", Version: "1.0.0"}}
	b := mod{Descriptor{ID: "sales", Version: "1.2.0", Dependencies: []Dependency{{ID: "auth", Major: 1}}}}
	got, e := Resolve(b, a)
	if e != nil || !reflect.DeepEqual([]string{got[0].Describe().ID, got[1].Describe().ID}, []string{"auth", "sales"}) {
		t.Fatalf("%v %v", got, e)
	}
	for _, ms := range [][]Module{{a, a}, {b}, {mod{Descriptor{ID: "auth", Version: "2.0.0"}}, b}, {mod{Descriptor{ID: "a", Version: "1.0.0", Dependencies: []Dependency{{ID: "b", Major: 1}}}}, mod{Descriptor{ID: "b", Version: "1.0.0", Dependencies: []Dependency{{ID: "a", Major: 1}}}}}} {
		if _, e := Resolve(ms...); e == nil {
			t.Fatal("invalid graph accepted")
		}
	}
}
