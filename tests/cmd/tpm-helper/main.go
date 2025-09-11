package main

import (
	"context"
	"crypto/sha1"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-tpm/tpm"
	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpm2/transport"

	"github.com/opencontainers/runc/libcontainer/vtpm"
)

const (
	OwnerAuth = "owner12345"
	SRKAuth   = "srk12345"
)

var (
	deviceVersion = flag.String("deviceVersion", "2", "version of TPM to use")
	devicePath    = flag.String("devicePath", "/dev/tpm0", "path to the device")
	deviceCommand = flag.String("deviceCommand", "random", "command to execute using TPM device")
	hashAlgo      = flag.String("hashAlgo", "sha256", "hash algo to use in read pcr")
	prcIndexes    = flag.String("pcrs", "16", "array of pcr indexes")
)

const (
	RandomDataLen = 16
)

func convertAlgNameToHashAlgoIDTPM2(algo string) tpm2.TPMAlgID {
	switch algo {
	case "sha1":
		return tpm2.TPMAlgSHA1
	case "sha256":
		return tpm2.TPMAlgSHA256
	case "sha384":
		return tpm2.TPMAlgSHA384
	case "sha512":
		return tpm2.TPMAlgSHA512
	default:
		return tpm2.TPMAlgNull
	}
}

func WorkWithTPM2(devicePath string, command string, dataCh chan []byte, errCh chan error) {

	var rwc io.ReadWriteCloser
	f, err := os.OpenFile(devicePath, os.O_RDWR, 0600)
	if err != nil {
		errCh <- fmt.Errorf("can not open tpm2 device path %s: %w", devicePath, err)
		return
	}
	rwc = io.ReadWriteCloser(f)

	tpmInterface := transport.FromReadWriter(rwc)
	switch command {
	case "random":
		cmd := tpm2.GetRandom{
			BytesRequested: RandomDataLen,
		}
		data, err := cmd.Execute(tpmInterface)
		if err != nil {
			errCh <- fmt.Errorf("Can not get random data from tpm2 device path %s: %+v", devicePath, err)
			return
		}
		dataCh <- data.RandomBytes.Buffer
	case "pcr":
		var pcrs []uint
		for _, indexStr := range strings.Split(*prcIndexes, ",") {
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				errCh <- fmt.Errorf("Can not parse PCR index %s: %w", indexStr, err)
				return
			}
			pcrs = append(pcrs, uint(index))
		}
		cmd := tpm2.PCRRead{
			PCRSelectionIn: tpm2.TPMLPCRSelection{
				PCRSelections: []tpm2.TPMSPCRSelection{
					{
						Hash:      convertAlgNameToHashAlgoIDTPM2(*hashAlgo),
						PCRSelect: tpm2.PCClientCompatible.PCRs(pcrs...),
					},
				},
			},
		}
		mp, err := cmd.Execute(tpmInterface)
		if err != nil {
			errCh <- fmt.Errorf("Can not get PCR data from tpm2 device path %s: %+v", devicePath, err)
			return
		}

		var data []byte

		for _, dt := range mp.PCRValues.Digests {
			data = append(data, dt.Buffer...)
		}

		if len(data) == 0 {
			errCh <- fmt.Errorf("selected pcrs are blanked")
			return
		}
		dataCh <- data

	case "pubek":
		var cmd tpm2.ReadPublic
		cmd.ObjectHandle = tpm2.TPMHandle(0x81010001)
		resp, err := cmd.Execute(tpmInterface)
		if err != nil {
			errCh <- fmt.Errorf("Can not get public ek data from tpm2 device path %s: %+v", devicePath, err)
			return
		}
		dataCh <- resp.OutPublic.Bytes()
	case "cert":
		var cmd tpm2.NVReadPublic
		cmd.NVIndex = tpm2.TPMHandle(0x01C00002)
		resp, err := cmd.Execute(tpmInterface)
		if err != nil {
			errCh <- fmt.Errorf("Can not get public ek cert from tpm2 device path %s: %+v", devicePath, err)
			return
		}
		dataCh <- resp.NVName.Buffer
	default:
		errCh <- fmt.Errorf("not defined command %s", command)
	}
}

func WorkWithTPM12(devicePath string, command string, dataCh chan []byte, errCh chan error) {
	rwc, err := tpm.OpenTPM(devicePath)
	if err != nil {
		errCh <- fmt.Errorf("Can not open tpm12 device path %s: %+v", devicePath, err)
		return
	}
	switch command {
	case "random":
		data, err := tpm.GetRandom(rwc, RandomDataLen)
		if err != nil {
			errCh <- fmt.Errorf("Can not get random data from tpm12 device path %s: %+v", devicePath, err)
			return
		}
		dataCh <- data
	case "pcr":
		var indexes []int
		for _, indexStr := range strings.Split(*prcIndexes, ",") {
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				errCh <- fmt.Errorf("Can not parse PCR index %s: %w", indexStr, err)
				return
			}
			indexes = append(indexes, index)
		}
		data, err := tpm.FetchPCRValues(rwc, indexes)
		if err != nil {
			errCh <- fmt.Errorf("Can not get PCR data from tpm 1.2 device path %s: %+v", devicePath, err)
			return
		}
		if len(data) == 0 {
			errCh <- fmt.Errorf("selected pcrs are blanked")
			return
		}
		dataCh <- data
	case "pubek":
		data, err := tpm.ReadPubEK(rwc)
		if err != nil {
			errCh <- fmt.Errorf("Can not get public ek data from tpm 1.2 device path %s: %+v", devicePath, err)
			return
		}
		dataCh <- data
	case "owner":
		data, err := tpm.ReadPubEK(rwc)
		if err != nil {
			errCh <- fmt.Errorf("Can not get public ek data from tpm 1.2 device path %s: %+v", devicePath, err)
			return
		}

		var ownerAuth [20]byte
		oa := sha1.Sum([]byte(OwnerAuth))
		copy(ownerAuth[:], oa[:])

		var srkAuth [20]byte
		sa := sha1.Sum([]byte(SRKAuth))
		copy(srkAuth[:], sa[:])

		err = tpm.TakeOwnership(rwc, ownerAuth, srkAuth, data)
		if err != nil {
			errCh <- fmt.Errorf("Can not get set ownership from tpm 1.2 device path %s: %+v", devicePath, err)
			return
		}

		dataCh <- []byte{}
	case "cert":
		var ownerAuth [20]byte
		oa := sha1.Sum([]byte(OwnerAuth))
		copy(ownerAuth[:], oa[:])

		data, err := tpm.ReadEKCert(rwc, ownerAuth)
		if err != nil {
			errCh <- fmt.Errorf("Can not get ek cert data from tpm 1.2 device path %s: %+v", devicePath, err)
			return
		}
		x509cert, err := x509.ParseCertificate(data)
		if err != nil {
			errCh <- fmt.Errorf("Can not get unmarshall x509 ek cert from tpm 1.2 device path %s: %+v", devicePath, err)
			return
		}
		dataCh <- x509cert.Raw
	default:
		errCh <- fmt.Errorf("not defined command %s", command)
	}
}

func main() {
	flag.Parse()
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	dataCh := make(chan []byte, RandomDataLen)
	errCh := make(chan error)
	switch *deviceVersion {
	case vtpm.VTPM_VERSION_2:
		go WorkWithTPM2(*devicePath, *deviceCommand, dataCh, errCh)
		select {
		case <-dataCh:
			break
		case err := <-errCh:
			log.Fatalf("Got an error while working with TPM2 device: %+v", err)
		case <-ctx.Done():
			log.Fatalf("Time exceed to get random from TPM2 device")
		}
	case vtpm.VTPM_VERSION_1_2:
		go WorkWithTPM12(*devicePath, *deviceCommand, dataCh, errCh)
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
