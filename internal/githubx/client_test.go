package githubx

import "testing"

func TestParseRemote(t *testing.T) {
	cases := map[string]Repo{"git@github.com:Rayfts/WhyThis.git": {Owner: "Rayfts", Name: "WhyThis"}, "https://github.com/Rayfts/WhyThis.git": {Owner: "Rayfts", Name: "WhyThis"}}
	for in, want := range cases {
		got, err := ParseRemote(in)
		if err != nil || got != want {
			t.Fatalf("%s => %#v %v", in, got, err)
		}
	}
}
