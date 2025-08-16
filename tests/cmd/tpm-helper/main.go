package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/google/go-tpm/legacy/tpm2"
	"github.com/google/go-tpm/tpm"
	"github.com/opencontainers/runc/libcontainer/vtpm"
)

var (
	deviceVersion = flag.String("deviceVersion", "2", "version of TPM to use")
	devicePath    = flag.String("devicePath", "/dev/tpm0", "path to the device")
)

const (
	RandomDataLen = 16
)

func WorkWithTPM2(devicePath string, dataCh chan []byte, errCh chan error) {
	rwc, err := tpm2.OpenTPM(devicePath)
	if err != nil {
		errCh <- fmt.Errorf("Can not open tpm2 device path %s: %+v", devicePath, err)
		return
	}
	data, err := tpm2.GetRandom(rwc, RandomDataLen)
	if err != nil {
		errCh <- fmt.Errorf("Can not get random data from tpm2 device path %s: %+v", devicePath, err)
		return
	}
	dataCh <- data
}

func WorkWithTPM12(devicePath string, dataCh chan []byte, errCh chan error) {
	rwc, err := tpm.OpenTPM(devicePath)
	if err != nil {
		errCh <- fmt.Errorf("Can not open tpm12 device path %s: %+v", devicePath, err)
		return
	}
	data, err := tpm.GetRandom(rwc, RandomDataLen)
	if err != nil {
		errCh <- fmt.Errorf("Can not get random data from tpm12 device path %s: %+v", devicePath, err)
		return
	}
	dataCh <- data
}

func main() {
	flag.Parse()
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	dataCh := make(chan []byte, RandomDataLen)
	errCh := make(chan error)
	switch *deviceVersion {
	case vtpm.VTPM_VERSION_2:
		go WorkWithTPM2(*devicePath, dataCh, errCh)
		select {
		case <-dataCh:
			break
		case err := <-errCh:
			log.Fatalf("Got an error while working with TPM2 device: %+v", err)
		case <-ctx.Done():
			log.Fatalf("Time exceed to get random from TPM2 device")
		}
	case vtpm.VTPM_VERSION_1_2:
		go WorkWithTPM12(*devicePath, dataCh, errCh)
		select {
		case <-dataCh:
			break
		case err := <-errCh:
			log.Fatalf("Got an error while working with TPM12 device: %+v", err)
		case <-ctx.Done():
			log.Fatalf("Time exceed to get random from TPM12 device")
		}
	default:
		log.Fatalf("Wrong TPM version %s", *deviceVersion)
	}

}
