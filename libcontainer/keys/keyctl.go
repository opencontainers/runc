// +build linux

package keyctl

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const KEYCTL_JOIN_SESSION_KEYRING = 1
const KEYCTL_SETPERM = 5
const KEYCTL_DESCRIBE = 6

type KeySerial uint32

func updateKeyringQuota(keyNameLen int) error {
	b, err := ioutil.ReadFile("/proc/key-users")
	if err != nil {
		return fmt.Errorf("failed to open /proc/key-users: %v", err)
	}
	str := string(b)
	str = strings.Trim(str, " \t")
	lines := strings.Split(str, "\n")
	for _, line := range lines {
		ar := strings.Split(line, " ")
		// idx 1 is uid
		// idx len - 1 is maxbyte quota
		// idx len - 2 is maxkeys count
		// we run as root, so only care about uid 0
		if fmt.Sprintf("%d:", 0) == ar[0] {
			var (
				path   string
				status []string
				cb     int64 // How many bytes are currently used by keys
				mb     int64 // Maximun allowed bytes for user
				ck     int64 // How many keys are in used for the user
				mk     int64 // Maximum allowed number of keys for user
				err    error
			)
			// Check maxbytes first
			status = strings.Split(ar[len(ar)-1], "/")
			if len(status) != 2 {
				return fmt.Errorf("unexpected format /proc/key-users format: %s", line)
			}
			cb, err = strconv.ParseInt(status[0], 10, 32)
			if err != nil {
				return fmt.Errorf("failed to convert current bytes value: %v", err)
			}
			mb, err = strconv.ParseInt(status[1], 10, 32)
			if err != nil {
				return fmt.Errorf("failed to convert max bytes value: %v", err)
			}

			// The kernel add 1 byte internally
			if cb == mb || cb+int64(keyNameLen)+1 >= mb {
				// Add 10% more quota
				mb = mb + (mb * 10 / 100)
				path = "/proc/sys/kernel/keys/root_maxbytes"
				err = ioutil.WriteFile(path, []byte(fmt.Sprintf("%d", uint32(mb))), 0644)
				if err != nil {
					return fmt.Errorf("failed to update %s: %v", path, err)
				}
			}

			// Now check maxkeys
			status = strings.Split(ar[len(ar)-2], "/")
			if len(status) != 2 {
				return fmt.Errorf("unexpected format /proc/key-users format: %s", line)
			}
			ck, err = strconv.ParseInt(status[0], 10, 32)
			if err != nil {
				return fmt.Errorf("failed to convert current key count value: %v", err)
			}
			mk, err = strconv.ParseInt(status[1], 10, 32)
			if err != nil {
				return fmt.Errorf("failed to convert max key count value: %v", err)
			}

			// On some machines the error was seen when 199/200 was
			// reached so add 1 for good measure
			if ck+1 >= mk {
				// Add 10% more quota
				mk = mk + (mk * 10 / 100)
				path = "/proc/sys/kernel/keys/root_maxkeys"
				err = ioutil.WriteFile(path, []byte(fmt.Sprintf("%d", uint32(mk))), 0644)
				if err != nil {
					return fmt.Errorf("failed to update %s: %v", path, err)
				}
			}

			break
		}
	}

	return nil
}

func JoinSessionKeyring(name string) (KeySerial, error) {
	var _name *byte = nil
	var err error

	if len(name) > 0 {
		_name, err = syscall.BytePtrFromString(name)
		if err != nil {
			return KeySerial(0), err
		}
	}

loop:
	sessKeyId, _, errn := syscall.Syscall(syscall.SYS_KEYCTL, KEYCTL_JOIN_SESSION_KEYRING, uintptr(unsafe.Pointer(_name)), 0)
	if errn == syscall.EDQUOT {
		if err := updateKeyringQuota(len(name)); err != nil {
			return KeySerial(0), fmt.Errorf("failed to raised keyring quota: %v", err)
		}
		goto loop
	}
	if errn != 0 {
		return 0, fmt.Errorf("could not create session key: %v", errn)
	}
	return KeySerial(sessKeyId), nil
}

// modify permissions on a keyring by reading the current permissions,
// anding the bits with the given mask (clearing permissions) and setting
// additional permission bits
func ModKeyringPerm(ringId KeySerial, mask, setbits uint32) error {
	dest := make([]byte, 1024)
	destBytes := unsafe.Pointer(&dest[0])

	if _, _, err := syscall.Syscall6(syscall.SYS_KEYCTL, uintptr(KEYCTL_DESCRIBE), uintptr(ringId), uintptr(destBytes), uintptr(len(dest)), 0, 0); err != 0 {
		return err
	}

	res := strings.Split(string(dest), ";")
	if len(res) < 5 {
		return fmt.Errorf("Destination buffer for key description is too small")
	}

	// parse permissions
	perm64, err := strconv.ParseUint(res[3], 16, 32)
	if err != nil {
		return err
	}

	perm := (uint32(perm64) & mask) | setbits

	if _, _, err := syscall.Syscall(syscall.SYS_KEYCTL, uintptr(KEYCTL_SETPERM), uintptr(ringId), uintptr(perm)); err != 0 {
		return err
	}

	return nil
}
