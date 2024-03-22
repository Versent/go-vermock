package vermockgen_test

import (
	"bytes"
	"flag"
	"log"
	"testing"

	vermockgen "github.com/Versent/go-vermock/internal/cmd/vermockgen"
)

func TestSetFlags(t *testing.T) {
	f := flag.NewFlagSet("vermockgen", flag.ContinueOnError)
	vermockgen.NewGenCmd(nil, f)
	expected := []string{"header", "tags"}
	f.VisitAll(func(f *flag.Flag) {
		if len(expected) == 0 {
			t.Errorf("unexpected flag %q", f.Name)
			return
		}

		var got, want string
		got, want, expected = f.Name, expected[0], expected[1:]
		if got != want {
			t.Errorf("unexpected name, got %q, want %q", got, want)
		}
	})
}

func TestExecute(t *testing.T) {
	var buf bytes.Buffer
	l := log.New(&buf, "", 0)
	f := flag.NewFlagSet("vermockgen", flag.ContinueOnError)
	vermockgen.NewGenCmd(l, f)

}
