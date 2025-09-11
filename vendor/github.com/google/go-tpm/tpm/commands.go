// Copyright (c) 2014, Google LLC All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tpm

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/google/go-tpm/tpmutil"
)

// submitTPMRequest sends a structure to the TPM device file and gets results
// back, interpreting them as a new provided structure.
func submitTPMRequest(rw io.ReadWriter, tag uint16, ord uint32, in []interface{}, out []interface{}) (uint32, error) {
	resp, code, err := tpmutil.RunCommand(rw, tpmutil.Tag(tag), tpmutil.Command(ord), in...)
	if err != nil {
		return 0, err
	}
	if code != tpmutil.RCSuccess {
		return uint32(code), tpmError(code)
	}

	_, err = tpmutil.Unpack(resp, out...)
	return 0, err
}

// oiap sends an OIAP command to the TPM and gets back an auth value and a
// nonce.
func oiap(rw io.ReadWriter) (*oiapResponse, error) {
	var resp oiapResponse
	out := []interface{}{&resp}
	// In this case, we don't need to check ret, since all the information is
	// contained in err.
	if _, err := submitTPMRequest(rw, tagRQUCommand, ordOIAP, nil, out); err != nil {
		return nil, err
	}

	return &resp, nil
}

// osap sends an OSAPCommand to the TPM and gets back authentication
// information in an OSAPResponse.
func osap(rw io.ReadWriter, osap *osapCommand) (*osapResponse, error) {
	in := []interface{}{osap}
	var resp osapResponse
	out := []interface{}{&resp}
	// In this case, we don't need to check the ret value, since all the
	// information is contained in err.
	if _, err := submitTPMRequest(rw, tagRQUCommand, ordOSAP, in, out); err != nil {
		return nil, err
	}

	return &resp, nil
}

// seal performs a seal operation on the TPM.
func seal(rw io.ReadWriter, sc *sealCommand, pcrs *pcrInfoLong, data tpmutil.U32Bytes, ca *commandAuth) (*tpmStoredData, *responseAuth, uint32, error) {
	pcrsize := binary.Size(pcrs)
	if pcrsize < 0 {
		return nil, nil, 0, errors.New("couldn't compute the size of a pcrInfoLong")
	}

	// TODO(tmroeder): special-case pcrInfoLong in pack/unpack so we don't have
	// to write out the length explicitly here.
	in := []interface{}{sc, uint32(pcrsize), pcrs, data, ca}

	var tsd tpmStoredData
	var ra responseAuth
	out := []interface{}{&tsd, &ra}
	ret, err := submitTPMRequest(rw, tagRQUAuth1Command, ordSeal, in, out)
	if err != nil {
		return nil, nil, 0, err
	}

	return &tsd, &ra, ret, nil
}

// unseal data sealed by the TPM.
func unseal(rw io.ReadWriter, keyHandle tpmutil.Handle, sealed *tpmStoredData, ca1 *commandAuth, ca2 *commandAuth) ([]byte, *responseAuth, *responseAuth, uint32, error) {
	in := []interface{}{keyHandle, sealed, ca1, ca2}
	var outb tpmutil.U32Bytes
	var ra1 responseAuth
	var ra2 responseAuth
	out := []interface{}{&outb, &ra1, &ra2}
	ret, err := submitTPMRequest(rw, tagRQUAuth2Command, ordUnseal, in, out)
	if err != nil {
		return nil, nil, nil, 0, err
	}

	return outb, &ra1, &ra2, ret, nil
}

// authorizeMigrationKey authorizes a public key for migrations.
func authorizeMigrationKey(rw io.ReadWriter, migrationScheme MigrationScheme, migrationKey pubKey, ca *commandAuth) ([]byte, *responseAuth, uint32, error) {
	in := []interface{}{migrationScheme, migrationKey, ca}
	var ra responseAuth
	var migrationAuth migrationKeyAuth
	out := []interface{}{&migrationAuth, &ra}
	ret, err := submitTPMRequest(rw, tagRQUAuth1Command, ordAuthorizeMigrationKey, in, out)
	if err != nil {
		return nil, nil, 0, err
	}
	authBlob, err := tpmutil.Pack(migrationAuth)
	if err != nil {
		return nil, nil, 0, err
	}

	return authBlob, &ra, ret, nil
}

// createMigrationBlob migrates a key from the TPM.
func createMigrationBlob(rw io.ReadWriter, parentHandle tpmutil.Handle, migrationScheme MigrationScheme, migrationKey []byte, encData tpmutil.U32Bytes, ca1 *commandAuth, ca2 *commandAuth) ([]byte, []byte, *responseAuth, *responseAuth, uint32, error) {
	in := []interface{}{parentHandle, migrationScheme, migrationKey, encData, ca1, ca2}
	var rand tpmutil.U32Bytes
	var outData tpmutil.U32Bytes
	var ra1 responseAuth
	var ra2 responseAuth
	out := []interface{}{&rand, &outData, &ra1, &ra2}
	ret, err := submitTPMRequest(rw, tagRQUAuth2Command, ordCreateMigrationBlob, in, out)
	if err != nil {
		return nil, nil, nil, nil, 0, err
	}

	return rand, outData, &ra1, &ra2, ret, nil
}

// flushSpecific removes a handle from the TPM. Note that removing a handle
// doesn't require any authentication.
func flushSpecific(rw io.ReadWriter, handle tpmutil.Handle, resourceType uint32) error {
	// In this case, all the information is in err, so we don't check the
	// specific return-value details.
	_, err := submitTPMRequest(rw, tagRQUCommand, ordFlushSpecific, []interface{}{handle, resourceType}, nil)
	return err
}

// loadKey2 loads a key into the TPM. It's a tagRQUAuth1Command, so it only
// needs one auth parameter.
// TODO(tmroeder): support key12, too.
func loadKey2(rw io.ReadWriter, k *key, ca *commandAuth) (tpmutil.Handle, *responseAuth, uint32, error) {
	// We always load our keys with the SRK as the parent key.
	in := []interface{}{khSRK, k, ca}
	var keyHandle tpmutil.Handle
	var ra responseAuth
	out := []interface{}{&keyHandle, &ra}
	ret, err := submitTPMRequest(rw, tagRQUAuth1Command, ordLoadKey2, in, out)
	if err != nil {
		return 0, nil, 0, err
	}

	return keyHandle, &ra, ret, nil
}

// getPubKey gets a public key from the TPM
func getPubKey(rw io.ReadWriter, keyHandle tpmutil.Handle, ca *commandAuth) (*pubKey, *responseAuth, uint32, error) {
	in := []interface{}{keyHandle, ca}
	var pk pubKey
	var ra responseAuth
	out := []interface{}{&pk, &ra}
	ret, err := submitTPMRequest(rw, tagRQUAuth1Command, ordGetPubKey, in, out)
	if err != nil {
		return nil, nil, 0, err
	}

	return &pk, &ra, ret, nil
}

// getCapability reads the requested capability and sub-capability from the TPM
func getCapability(rw io.ReadWriter, cap, subcap uint32) ([]byte, error) {
	subCapBytes, err := tpmutil.Pack(subcap)
	if err != nil {
		return nil, err
	}
	var b tpmutil.U32Bytes
	in := []interface{}{cap, tpmutil.U32Bytes(subCapBytes)}
	out := []interface{}{&b}
	if _, err := submitTPMRequest(rw, tagRQUCommand, ordGetCapability, in, out); err != nil {
		return nil, err
	}
	return b, nil
}

// nvDefineSpace allocates space in NVRAM
func nvDefineSpace(rw io.ReadWriter, nvData NVDataPublic, enc Digest, ca *commandAuth) (*responseAuth, uint32, error) {
	var ra responseAuth
	in := []interface{}{nvData, enc}
	if ca != nil {
		in = append(in, ca)
	}
	out := []interface{}{&ra}
	ret, err := submitTPMRequest(rw, tagRQUAuth1Command, ordNVDefineSpace, in, out)
	if err != nil {
		return nil, 0, err
	}
	return &ra, ret, nil

}

// nvReadValue reads from the NVRAM
// If TPM isn't locked, and for some nv permission no authentication is needed.
// See TPM-Main-Part-3-Commands-20.4
func nvReadValue(rw io.ReadWriter, index, offset, len uint32, ca *commandAuth) ([]byte, *responseAuth, uint32, error) {
	var b tpmutil.U32Bytes
	var ra responseAuth
	var ret uint32
	var err error
	in := []interface{}{index, offset, len}
	out := []interface{}{&b}
	// Auth is needed
	if ca != nil {
		in = append(in, ca)
		out = append(out, &ra)
		ret, err = submitTPMRequest(rw, tagRQUAuth1Command, ordNVReadValue, in, out)
	} else {
		// Auth is not needed
		ret, err = submitTPMRequest(rw, tagRQUCommand, ordNVReadValue, in, out)
	}
	if err != nil {
		return nil, nil, 0, err
	}
	return b, &ra, ret, nil
}

// nvReadValueAuth reads nvram with enforced authentication.
// No Owner needs to be present.
// See TPM-Main-Part-3-Commands-20.5
func nvReadValueAuth(rw io.ReadWriter, index, offset, len uint32, ca *commandAuth) ([]byte, *responseAuth, uint32, error) {
	var b tpmutil.U32Bytes
	var ra responseAuth
	in := []interface{}{index, offset, len, ca}
	out := []interface{}{&b, &ra}
	ret, err := submitTPMRequest(rw, tagRQUAuth1Command, ordNVReadValueAuth, in, out)
	if err != nil {
		return nil, nil, 0, err
	}
	return b, &ra, ret, nil
}

// nvWriteValue writes to the NVRAM
// If TPM isn't locked, no authentication is needed.
// See TPM-Main-Part-3-Commands-20.2
func nvWriteValue(rw io.ReadWriter, index, offset, len uint32, data []byte, ca *commandAuth) ([]byte, *responseAuth, uint32, error) {
	var b tpmutil.U32Bytes
	var ra responseAuth
	in := []interface{}{index, offset, len, data}
	if ca != nil {
		in = append(in, ca)
	}
	out := []interface{}{&b, &ra}
	ret, err := submitTPMRequest(rw, tagRQUAuth1Command, ordNVWriteValue, in, out)
	if err != nil {
		return nil, nil, 0, err
	}
	return b, &ra, ret, nil
}

// nvWriteValue writes to the NVRAM
// If TPM isn't locked, no authentication is needed.
// See TPM-Main-Part-3-Commands-20.3
func nvWriteValueAuth(rw io.ReadWriter, index, offset, len uint32, data []byte, ca *commandAuth) ([]byte, *responseAuth, uint32, error) {
	var b tpmutil.U32Bytes
	var ra responseAuth
	in := []interface{}{index, offset, len, data, ca}
	out := []interface{}{&b, &ra}
	ret, err := submitTPMRequest(rw, tagRQUAuth1Command, ordNVWriteValueAuth, in, out)
	if err != nil {
		return nil, nil, 0, err
	}
	return b, &ra, ret, nil
}

// quote2 signs arbitrary data under a given set of PCRs and using a key
// specified by keyHandle. It returns information about the PCRs it signed
// under, the signature, auth information, and optionally information about the
// TPM itself. Note that the input to quote2 must be exactly 20 bytes, so it is
// normally the SHA1 hash of the data.
func quote2(rw io.ReadWriter, keyHandle tpmutil.Handle, hash [20]byte, pcrs *pcrSelection, addVersion byte, ca *commandAuth) (*pcrInfoShort, *CapVersionInfo, []byte, []byte, *responseAuth, uint32, error) {
	in := []interface{}{keyHandle, hash, pcrs, addVersion, ca}
	var pcrShort pcrInfoShort
	var capInfo CapVersionInfo
	var capBytes tpmutil.U32Bytes
	var sig tpmutil.U32Bytes
	var ra responseAuth
	out := []interface{}{&pcrShort, &capBytes, &sig, &ra}
	ret, err := submitTPMRequest(rw, tagRQUAuth1Command, ordQuote2, in, out)
	if err != nil {
		return nil, nil, nil, nil, nil, 0, err
	}

	// Deserialize the capInfo, if any.
	if len([]byte(capBytes)) == 0 {
		return &pcrShort, nil, capBytes, sig, &ra, ret, nil
	}

	err = capInfo.Decode(capBytes)
	if err != nil {
		return nil, nil, nil, nil, nil, 0, err
	}

	return &pcrShort, &capInfo, capBytes, sig, &ra, ret, nil
}

// quote performs a TPM 1.1 quote operation: it signs data using the
// TPM_QUOTE_INFO structure for the current values of a selected set of PCRs.
func quote(rw io.ReadWriter, keyHandle tpmutil.Handle, hash [20]byte, pcrs *pcrSelection, ca *commandAuth) (*pcrComposite, []byte, *responseAuth, uint32, error) {
	in := []interface{}{keyHandle, hash, pcrs, ca}
	var pcrc pcrComposite
	var sig tpmutil.U32Bytes
	var ra responseAuth
	out := []interface{}{&pcrc, &sig, &ra}
	ret, err := submitTPMRequest(rw, tagRQUAuth1Command, ordQuote, in, out)
	if err != nil {
		return nil, nil, nil, 0, err
	}

	return &pcrc, sig, &ra, ret, nil
}

// makeIdentity requests that the TPM create a new AIK. It returns the handle to
// this new key.
func makeIdentity(rw io.ReadWriter, encAuth Digest, idDigest Digest, k *key, ca1 *commandAuth, ca2 *commandAuth) (*key, []byte, *responseAuth, *responseAuth, uint32, error) {
	in := []interface{}{encAuth, idDigest, k, ca1, ca2}
	var aik key
	var sig tpmutil.U32Bytes
	var ra1 responseAuth
	var ra2 responseAuth
	out := []interface{}{&aik, &sig, &ra1, &ra2}
	ret, err := submitTPMRequest(rw, tagRQUAuth2Command, ordMakeIdentity, in, out)
	if err != nil {
		return nil, nil, nil, nil, 0, err
	}

	return &aik, sig, &ra1, &ra2, ret, nil
}

// activateIdentity provides the TPM with an EK encrypted challenge and asks it to
// decrypt the challenge and return the secret (symmetric key).
func activateIdentity(rw io.ReadWriter, keyHandle tpmutil.Handle, blob tpmutil.U32Bytes, ca1 *commandAuth, ca2 *commandAuth) (*symKey, *responseAuth, *responseAuth, uint32, error) {
	in := []interface{}{keyHandle, blob, ca1, ca2}
	var symkey symKey
	var ra1 responseAuth
	var ra2 responseAuth
	out := []interface{}{&symkey, &ra1, &ra2}
	ret, err := submitTPMRequest(rw, tagRQUAuth2Command, ordActivateIdentity, in, out)
	if err != nil {
		return nil, nil, nil, 0, err
	}

	return &symkey, &ra1, &ra2, ret, nil
}

// resetLockValue resets the dictionary-attack lock in the TPM, using owner
// auth.
func resetLockValue(rw io.ReadWriter, ca *commandAuth) (*responseAuth, uint32, error) {
	in := []interface{}{ca}
	var ra responseAuth
	out := []interface{}{&ra}
	ret, err := submitTPMRequest(rw, tagRQUAuth1Command, ordResetLockValue, in, out)
	if err != nil {
		return nil, 0, err
	}

	return &ra, ret, nil
}

// ownerReadInternalPub uses owner auth and OSAP to read either the endorsement
// key (using khEK) or the SRK (using khSRK).
func ownerReadInternalPub(rw io.ReadWriter, kh tpmutil.Handle, ca *commandAuth) (*pubKey, *responseAuth, uint32, error) {
	in := []interface{}{kh, ca}
	var pk pubKey
	var ra responseAuth
	out := []interface{}{&pk, &ra}
	ret, err := submitTPMRequest(rw, tagRQUAuth1Command, ordOwnerReadInternalPub, in, out)
	if err != nil {
		return nil, nil, 0, err
	}

	return &pk, &ra, ret, nil
}

// readPubEK requests the public part of the endorsement key from the TPM. Note
// that this call can only be made when there is no owner in the TPM. Once an
// owner is established, the endorsement key can be retrieved using
// ownerReadInternalPub.
func readPubEK(rw io.ReadWriter, n Nonce) (*pubKey, Digest, uint32, error) {
	in := []interface{}{n}
	var pk pubKey
	var d Digest
	out := []interface{}{&pk, &d}
	ret, err := submitTPMRequest(rw, tagRQUCommand, ordReadPubEK, in, out)
	if err != nil {
		return nil, d, 0, err
	}

	return &pk, d, ret, nil
}

// ownerClear uses owner auth to clear the TPM. After this operation, a caller
// can take ownership of the TPM with TPM_TakeOwnership.
func ownerClear(rw io.ReadWriter, ca *commandAuth) (*responseAuth, uint32, error) {
	in := []interface{}{ca}
	var ra responseAuth
	out := []interface{}{&ra}
	ret, err := submitTPMRequest(rw, tagRQUAuth1Command, ordOwnerClear, in, out)
	if err != nil {
		return nil, 0, err
	}

	return &ra, ret, nil
}

// takeOwnership takes ownership of the TPM and establishes a new SRK and
// owner auth. This operation can only be performed if there is no owner. The
// TPM can be put into this state using TPM_OwnerClear. The encOwnerAuth and
// encSRKAuth values must be encrypted using the endorsement key.
func takeOwnership(rw io.ReadWriter, encOwnerAuth tpmutil.U32Bytes, encSRKAuth tpmutil.U32Bytes, srk *key, ca *commandAuth) (*key, *responseAuth, uint32, error) {
	in := []interface{}{pidOwner, encOwnerAuth, encSRKAuth, srk, ca}
	var k key
	var ra responseAuth
	out := []interface{}{&k, &ra}
	ret, err := submitTPMRequest(rw, tagRQUAuth1Command, ordTakeOwnership, in, out)
	if err != nil {
		return nil, nil, 0, err
	}

	return &k, &ra, ret, nil
}

// Creates a wrapped key under the SRK.
func createWrapKey(rw io.ReadWriter, encUsageAuth Digest, encMigrationAuth Digest, keyInfo *key, ca *commandAuth) (*key, *responseAuth, uint32, error) {
	in := []interface{}{khSRK, encUsageAuth, encMigrationAuth, keyInfo, ca}
	var k key
	var ra responseAuth
	out := []interface{}{&k, &ra}
	ret, err := submitTPMRequest(rw, tagRQUAuth1Command, ordCreateWrapKey, in, out)
	if err != nil {
		return nil, nil, 0, err
	}

	return &k, &ra, ret, nil
}

func sign(rw io.ReadWriter, keyHandle tpmutil.Handle, data tpmutil.U32Bytes, ca *commandAuth) ([]byte, *responseAuth, uint32, error) {
	in := []interface{}{keyHandle, data, ca}
	var signature tpmutil.U32Bytes
	var ra responseAuth
	out := []interface{}{&signature, &ra}
	ret, err := submitTPMRequest(rw, tagRQUAuth1Command, ordSign, in, out)
	if err != nil {
		return nil, nil, 0, err
	}

	return signature, &ra, ret, nil
}

func pcrReset(rw io.ReadWriter, pcrs *pcrSelection) error {
	_, err := submitTPMRequest(rw, tagRQUCommand, ordPcrReset, []interface{}{pcrs}, nil)
	if err != nil {
		return err
	}
	return nil
}
