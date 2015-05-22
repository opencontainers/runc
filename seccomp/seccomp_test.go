package seccomp

import (
	"fmt"
	"os/exec"
	"testing"
)

var osec = "/go/src/seccomp_main.go"

func secMain(t *testing.T, args []string) {
	if len(args) < 1 {
		return
	}

	cmd := args[0]
	path := "go"
	argv := []string{"run", osec}
	argv = append(argv, args[0:]...)

	c := exec.Command(path, argv...)
	_, err := c.Output()
	fmt.Printf("do %s, err is [%v]\n", cmd, err)
	if err != nil {
		if "writeOk" == cmd || "socketOk" == cmd {
			t.Fatal(err)
		}
	} else {
		if "writeErr" == cmd || "socketErr" == cmd {
			t.Fatal(err)
		}
	}
}

func commandGC(file string) {
	c := exec.Command("rm", "-rf", file)
	d, _ := c.Output()
	fmt.Println(string(d))
}

func cp(src, dst string) {
	c := exec.Command("cp", "-ra", src, dst)
	d, _ := c.Output()
	fmt.Println(string(d))
}

func TestSeccomp(t *testing.T) {
	//hard code
	cp("../seccomp", "/go/src/")
	cp("./seccomp.test", osec)
	defer commandGC("/go/src/seccomp")
	defer commandGC(osec)

	secMain(t, []string{"writeOk"})
	secMain(t, []string{"writeErr"})
	secMain(t, []string{"socketOk"})
	secMain(t, []string{"socketErr"})
}
