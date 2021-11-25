package main

import (
	"fmt"
	"os"
	"os/exec"
)

var rules bool = false

func main() {
	var debug bool = false
	if len(os.Args) > 1 {
		for i := 0; i < len(os.Args); i++ {
			switch os.Args[i] {
			case "--debug":
				debug = true
			case "--clips":
				rules = true
			}
		}
		if os.Args[1] == "1" {
			debug = true
		}
	}
	if debug {
		fmt.Println("Debug mode:")
	} else {
		m, tprcc = auto_config()
		pcCola = make(chan string, m["me"].slots)
		if m["me"].tipo != "pi" {
			cmd := exec.Command("iperf", "-s")
			err := cmd.Start()
			Check(err)
			defer cmd.Process.Kill()
			send_info()
			Server_tcp(handle)
		} else {
			go Server_tcp(handle)
			genTrafic()
		}
	}
}
