package tctbf

/*
extern int add_tc_tbf(int,int);
*/
import "C"

import (
	"errors"
	"net"
)

func AddTcTbf(hostInterfaceName string, networkSpeedLimit int) error {
	findNetInterfaces := false
	interfaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	i := net.Interface{}
	for _, i = range interfaces {
		if i.Name == hostInterfaceName {
			findNetInterfaces = true
			break
		}
	}
	if !findNetInterfaces {
		return errors.New(hostInterfaceName + " does not exist")
	}
	status := C.add_tc_tbf(C.int(i.Index), C.int(networkSpeedLimit))
	if status != 0 {
		return errors.New(hostInterfaceName + " Add Network Speed Limit Fail")
	}
	return nil
}
