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

// Package tpm supports direct communication with a tpm device under Linux.
package tpm

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/google/go-tpm/tpmutil"
)

// GetKeys gets the list of handles for currently-loaded TPM keys.
func GetKeys(rw io.ReadWriter) ([]tpmutil.Handle, error) {
	b, err := getCapability(rw, CapHandle, rtKey)
	if err != nil {
		return nil, err
	}
	var handles []tpmutil.Handle
	if _, err := tpmutil.Unpack(b, &handles); err != nil {
		return nil, err
	}
	return handles, err
}

// PcrExtend extends a value into the right PCR by index.
func PcrExtend(rw io.ReadWriter, pcrIndex uint32, pcr pcrValue) ([]byte, error) {
	in := []interface{}{pcrIndex, pcr}
	var d pcrValue
	out := []interface{}{&d}
	if _, err := submitTPMRequest(rw, tagRQUCommand, ordExtend, in, out); err != nil {
		return nil, err
	}

	return d[:], nil
}

// ReadPCR reads a PCR value from the TPM.
func ReadPCR(rw io.ReadWriter, pcrIndex uint32) ([]byte, error) {
	in := []interface{}{pcrIndex}
	var v pcrValue
	out := []interface{}{&v}
	// There's no need to check the ret value here, since the err value contains
	// all the necessary information.
	if _, err := submitTPMRequest(rw, tagRQUCommand, ordPCRRead, in, out); err != nil {
		return nil, err
	}

	return v[:], nil
}

// FetchPCRValues gets a given sequence of PCR values.
func FetchPCRValues(rw io.ReadWriter, pcrVals []int) ([]byte, error) {
	var pcrs []byte
	for _, v := range pcrVals {
		pcr, err := ReadPCR(rw, uint32(v))
		if err != nil {
			return nil, err
		}

		pcrs = append(pcrs, pcr...)
	}

	return pcrs, nil
}

// GetRandom gets random bytes from the TPM.
func GetRandom(rw io.ReadWriter, size uint32) ([]byte, error) {
	var b tpmutil.U32Bytes
	in := []interface{}{size}
	out := []interface{}{&b}
	// There's no need to check the ret value here, since the err value
	// contains all the necessary information.
	if _, err := submitTPMRequest(rw, tagRQUCommand, ordGetRandom, in, out); err != nil {
		return nil, err
	}

	return b, nil
}

// LoadKey2 loads a key blob (a serialized TPM_KEY or TPM_KEY12) into the TPM
// and returns a handle for this key.
func LoadKey2(rw io.ReadWriter, keyBlob []byte, srkAuth []byte) (tpmutil.Handle, error) {
	// Deserialize the keyBlob as a key
	var k key
	if _, err := tpmutil.Unpack(keyBlob, &k); err != nil {
		return 0, err
	}

	// Run OSAP for the SRK, reading a random OddOSAP for our initial
	// command and getting back a secret and a handle. LoadKey2 needs an
	// OSAP session for the SRK because the private part of a TPM_KEY or
	// TPM_KEY12 is sealed against the SRK.
	sharedSecret, osapr, err := newOSAPSession(rw, etSRK, khSRK, srkAuth)
	if err != nil {
		return 0, err
	}
	defer osapr.Close(rw)
	defer zeroBytes(sharedSecret[:])

	authIn := []interface{}{ordLoadKey2, k}
	ca, err := newCommandAuth(osapr.AuthHandle, osapr.NonceEven, nil, sharedSecret[:], authIn)
	if err != nil {
		return 0, err
	}

	handle, ra, ret, err := loadKey2(rw, &k, ca)
	if err != nil {
		return 0, err
	}

	// Check the response authentication.
	raIn := []interface{}{ret, ordLoadKey2}
	if err := ra.verify(ca.NonceOdd, sharedSecret[:], raIn); err != nil {
		return 0, err
	}

	return handle, nil
}

// Quote2 performs a quote operation on the TPM for the given data,
// under the key associated with the handle and for the pcr values
// specified in the call.
func Quote2(rw io.ReadWriter, handle tpmutil.Handle, data []byte, pcrVals []int, addVersion byte, aikAuth []byte) ([]byte, error) {
	// Run OSAP for the handle, reading a random OddOSAP for our initial
	// command and getting back a secret and a response.
	sharedSecret, osapr, err := newOSAPSession(rw, etKeyHandle, handle, aikAuth)
	if err != nil {
		return nil, err
	}
	defer osapr.Close(rw)
	defer zeroBytes(sharedSecret[:])

	// Hash the data to get the value to pass to quote2.
	hash := sha1.Sum(data)
	pcrSel, err := newPCRSelection(pcrVals)
	if err != nil {
		return nil, err
	}
	authIn := []interface{}{ordQuote2, hash, pcrSel, addVersion}
	ca, err := newCommandAuth(osapr.AuthHandle, osapr.NonceEven, nil, sharedSecret[:], authIn)
	if err != nil {
		return nil, err
	}

	// TODO(tmroeder): use the returned CapVersion.
	pcrShort, _, capBytes, sig, ra, ret, err := quote2(rw, handle, hash, pcrSel, addVersion, ca)
	if err != nil {
		return nil, err
	}

	// Check response authentication.
	raIn := []interface{}{ret, ordQuote2, pcrShort, tpmutil.U32Bytes(capBytes), tpmutil.U32Bytes(sig)}
	if err := ra.verify(ca.NonceOdd, sharedSecret[:], raIn); err != nil {
		return nil, err
	}

	return sig, nil
}

// GetPubKey retrieves an opaque blob containing a public key corresponding to
// a handle from the TPM.
func GetPubKey(rw io.ReadWriter, keyHandle tpmutil.Handle, srkAuth []byte) ([]byte, error) {
	// Run OSAP for the handle, reading a random OddOSAP for our initial
	// command and getting back a secret and a response.
	sharedSecret, osapr, err := newOSAPSession(rw, etKeyHandle, keyHandle, srkAuth)
	if err != nil {
		return nil, err
	}
	defer osapr.Close(rw)
	defer zeroBytes(sharedSecret[:])

	authIn := []interface{}{ordGetPubKey}
	ca, err := newCommandAuth(osapr.AuthHandle, osapr.NonceEven, nil, sharedSecret[:], authIn)
	if err != nil {
		return nil, err
	}

	pk, ra, ret, err := getPubKey(rw, keyHandle, ca)
	if err != nil {
		return nil, err
	}

	// Check response authentication for TPM_GetPubKey.
	raIn := []interface{}{ret, ordGetPubKey, pk}
	if err := ra.verify(ca.NonceOdd, sharedSecret[:], raIn); err != nil {
		return nil, err
	}

	b, err := tpmutil.Pack(*pk)
	if err != nil {
		return nil, err
	}
	return b, err
}

// newOSAPSession starts a new OSAP session and derives a shared key from it.
func newOSAPSession(rw io.ReadWriter, entityType uint16, entityValue tpmutil.Handle, srkAuth []byte) ([20]byte, *osapResponse, error) {
	osapc := &osapCommand{
		EntityType:  entityType,
		EntityValue: entityValue,
	}

	var sharedSecret [20]byte
	if _, err := rand.Read(osapc.OddOSAP[:]); err != nil {
		return sharedSecret, nil, err
	}

	osapr, err := osap(rw, osapc)
	if err != nil {
		return sharedSecret, nil, err
	}

	// A shared secret is computed as
	//
	// sharedSecret = HMAC-SHA1(srkAuth, evenosap||oddosap)
	//
	// where srkAuth is the hash of the SRK authentication (which hash is all 0s
	// for the well-known SRK auth value) and even and odd OSAP are the
	// values from the OSAP protocol.
	osapData, err := tpmutil.Pack(osapr.EvenOSAP, osapc.OddOSAP)
	if err != nil {
		return sharedSecret, nil, err
	}

	hm := hmac.New(sha1.New, srkAuth)
	hm.Write(osapData)
	// Note that crypto/hash.Sum returns a slice rather than an array, so we
	// have to copy this into an array to make sure that serialization doesn't
	// prepend a length in tpmutil.Pack().
	sharedSecretBytes := hm.Sum(nil)
	copy(sharedSecret[:], sharedSecretBytes)
	return sharedSecret, osapr, nil
}

// newCommandAuth creates a new commandAuth structure over the given
// parameters, using the given secret and the given odd nonce, if provided,
// for the HMAC. If no odd nonce is provided, one is randomly generated.
func newCommandAuth(authHandle tpmutil.Handle, nonceEven Nonce, nonceOdd *Nonce, key []byte, params []interface{}) (*commandAuth, error) {
	// Auth = HMAC-SHA1(key, SHA1(params) || NonceEven || NonceOdd || ContSession)
	digestBytes, err := tpmutil.Pack(params...)
	if err != nil {
		return nil, err
	}
	digest := sha1.Sum(digestBytes)

	// Use the passed-in nonce if non-nil, otherwise generate it now.
	var odd Nonce
	if nonceOdd != nil {
		odd = *nonceOdd
	} else {
		if _, err := rand.Read(odd[:]); err != nil {
			return nil, err
		}
	}

	ca := &commandAuth{
		AuthHandle: authHandle,
		NonceOdd:   odd,
	}

	authBytes, err := tpmutil.Pack(digest, nonceEven, ca.NonceOdd, ca.ContSession)
	if err != nil {
		return nil, err
	}

	hm2 := hmac.New(sha1.New, key)
	hm2.Write(authBytes)
	auth := hm2.Sum(nil)
	copy(ca.Auth[:], auth[:])
	return ca, nil
}

// verify checks that the response authentication was correct.
// It computes the SHA1 of params, and computes the HMAC-SHA1 of this digest
// with the authentication parameters of ra along with the given odd nonce.
func (ra *responseAuth) verify(nonceOdd Nonce, key []byte, params []interface{}) error {
	// Auth = HMAC-SHA1(key, SHA1(params) || ra.NonceEven || NonceOdd || ra.ContSession)
	digestBytes, err := tpmutil.Pack(params...)
	if err != nil {
		return err
	}

	digest := sha1.Sum(digestBytes)
	authBytes, err := tpmutil.Pack(digest, ra.NonceEven, nonceOdd, ra.ContSession)
	if err != nil {
		return err
	}

	hm2 := hmac.New(sha1.New, key)
	hm2.Write(authBytes)
	auth := hm2.Sum(nil)

	if !hmac.Equal(ra.Auth[:], auth) {
		return errors.New("the computed response HMAC didn't match the provided HMAC")
	}

	return nil
}

// zeroBytes zeroes a byte array.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func sealHelper(rw io.ReadWriter, pcrInfo *pcrInfoLong, data []byte, srkAuth []byte) ([]byte, error) {
	// Run OSAP for the SRK, reading a random OddOSAP for our initial
	// command and getting back a secret and a handle.
	sharedSecret, osapr, err := newOSAPSession(rw, etSRK, khSRK, srkAuth)
	if err != nil {
		return nil, err
	}
	defer osapr.Close(rw)
	defer zeroBytes(sharedSecret[:])

	// EncAuth for a seal command is computed as
	//
	// encAuth = XOR(srkAuth, SHA1(sharedSecret || <lastEvenNonce>))
	//
	// In this case, the last even nonce is NonceEven from OSAP.
	xorData, err := tpmutil.Pack(sharedSecret, osapr.NonceEven)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(xorData)

	encAuthData := sha1.Sum(xorData)
	sc := &sealCommand{KeyHandle: khSRK}
	for i := range sc.EncAuth {
		sc.EncAuth[i] = srkAuth[i] ^ encAuthData[i]
	}

	// The digest input for seal authentication is
	//
	// digest = SHA1(ordSeal || encAuth || binary.Size(pcrInfo) || pcrInfo ||
	//               len(data) || data)
	//
	authIn := []interface{}{ordSeal, sc.EncAuth, uint32(binary.Size(pcrInfo)), pcrInfo, tpmutil.U32Bytes(data)}
	ca, err := newCommandAuth(osapr.AuthHandle, osapr.NonceEven, nil, sharedSecret[:], authIn)
	if err != nil {
		return nil, err
	}

	sealed, ra, ret, err := seal(rw, sc, pcrInfo, data, ca)
	if err != nil {
		return nil, err
	}

	// Check the response authentication.
	raIn := []interface{}{ret, ordSeal, sealed}
	if err := ra.verify(ca.NonceOdd, sharedSecret[:], raIn); err != nil {
		return nil, err
	}

	sealedBytes, err := tpmutil.Pack(*sealed)
	if err != nil {
		return nil, err
	}

	return sealedBytes, nil
}

// Seal encrypts data against a given locality and PCRs and returns the sealed data.
func Seal(rw io.ReadWriter, loc Locality, pcrs []int, data []byte, srkAuth []byte) ([]byte, error) {
	pcrInfo, err := newPCRInfoLong(rw, loc, pcrs)
	if err != nil {
		return nil, err
	}
	return sealHelper(rw, pcrInfo, data, srkAuth)
}

// Reseal takes a pre-calculated PCR map and locality in order to seal data
// with a srkAuth. This function is necessary for PCR pre-calculation and later
// sealing to provide a way of updating software which is part of a measured
// boot process.
func Reseal(rw io.ReadWriter, loc Locality, pcrs map[int][]byte, data []byte, srkAuth []byte) ([]byte, error) {
	pcrInfo, err := newPCRInfoLongWithHashes(loc, pcrs)
	if err != nil {
		return nil, err
	}
	return sealHelper(rw, pcrInfo, data, srkAuth)
}

// Unseal decrypts data encrypted by the TPM.
func Unseal(rw io.ReadWriter, sealed []byte, srkAuth []byte) ([]byte, error) {
	// Run OSAP for the SRK, reading a random OddOSAP for our initial
	// command and getting back a secret and a handle.
	sharedSecret, osapr, err := newOSAPSession(rw, etSRK, khSRK, srkAuth)
	if err != nil {
		return nil, err
	}
	defer osapr.Close(rw)
	defer zeroBytes(sharedSecret[:])

	// The unseal command needs an OIAP session in addition to the OSAP session.
	oiapr, err := oiap(rw)
	if err != nil {
		return nil, err
	}
	defer oiapr.Close(rw)

	// Convert the sealed value into a tpmStoredData.
	var tsd tpmStoredData
	if _, err := tpmutil.Unpack(sealed, &tsd); err != nil {
		return nil, errors.New("couldn't convert the sealed data into a tpmStoredData struct")
	}

	// The digest for auth1 and auth2 for the unseal command is computed as
	// digest = SHA1(ordUnseal || tsd)
	authIn := []interface{}{ordUnseal, tsd}

	// The first commandAuth uses the shared secret as an HMAC key.
	ca1, err := newCommandAuth(osapr.AuthHandle, osapr.NonceEven, nil, sharedSecret[:], authIn)
	if err != nil {
		return nil, err
	}

	// The second commandAuth is based on OIAP instead of OSAP and uses the
	// SRK auth value as an HMAC key instead of the shared secret.
	ca2, err := newCommandAuth(oiapr.AuthHandle, oiapr.NonceEven, nil, srkAuth, authIn)
	if err != nil {
		return nil, err
	}

	unsealed, ra1, ra2, ret, err := unseal(rw, khSRK, &tsd, ca1, ca2)
	if err != nil {
		return nil, err
	}

	// Check the response authentication.
	raIn := []interface{}{ret, ordUnseal, tpmutil.U32Bytes(unsealed)}
	if err := ra1.verify(ca1.NonceOdd, sharedSecret[:], raIn); err != nil {
		return nil, err
	}

	if err := ra2.verify(ca2.NonceOdd, srkAuth, raIn); err != nil {
		return nil, err
	}

	return unsealed, nil
}

// Quote produces a TPM quote for the given data under the given PCRs. It uses
// AIK auth and a given AIK handle.
func Quote(rw io.ReadWriter, handle tpmutil.Handle, data []byte, pcrNums []int, aikAuth []byte) ([]byte, []byte, error) {
	// Run OSAP for the handle, reading a random OddOSAP for our initial
	// command and getting back a secret and a response.
	sharedSecret, osapr, err := newOSAPSession(rw, etKeyHandle, handle, aikAuth)
	if err != nil {
		return nil, nil, err
	}
	defer osapr.Close(rw)
	defer zeroBytes(sharedSecret[:])

	// Hash the data to get the value to pass to quote2.
	hash := sha1.Sum(data)
	pcrSel, err := newPCRSelection(pcrNums)
	if err != nil {
		return nil, nil, err
	}
	authIn := []interface{}{ordQuote, hash, pcrSel}
	ca, err := newCommandAuth(osapr.AuthHandle, osapr.NonceEven, nil, sharedSecret[:], authIn)
	if err != nil {
		return nil, nil, err
	}

	pcrc, sig, ra, ret, err := quote(rw, handle, hash, pcrSel, ca)
	if err != nil {
		return nil, nil, err
	}

	// Check response authentication.
	raIn := []interface{}{ret, ordQuote, pcrc, tpmutil.U32Bytes(sig)}
	if err := ra.verify(ca.NonceOdd, sharedSecret[:], raIn); err != nil {
		return nil, nil, err
	}

	return sig, pcrc.Values, nil
}

// MakeIdentity creates a new AIK with the given new auth value, and the given
// parameters for the privacy CA that will be used to attest to it.
// If both pk and label are nil, then the TPM_CHOSENID_HASH is set to all 0s as
// a special case. MakeIdentity returns a key blob for the newly-created key.
// The caller must be authorized to use the SRK, since the private part of the
// AIK is sealed against the SRK.
// TODO(tmroeder): currently, this code can only create 2048-bit RSA keys.
func MakeIdentity(rw io.ReadWriter, srkAuth []byte, ownerAuth []byte, aikAuth []byte, pk crypto.PublicKey, label []byte) ([]byte, error) {
	// Run OSAP for the SRK, reading a random OddOSAP for our initial command
	// and getting back a secret and a handle.
	sharedSecretSRK, osaprSRK, err := newOSAPSession(rw, etSRK, khSRK, srkAuth)
	if err != nil {
		return nil, err
	}
	defer osaprSRK.Close(rw)
	defer zeroBytes(sharedSecretSRK[:])

	// Run OSAP for the Owner, reading a random OddOSAP for our initial command
	// and getting back a secret and a handle.
	sharedSecretOwn, osaprOwn, err := newOSAPSession(rw, etOwner, khOwner, ownerAuth)
	if err != nil {
		return nil, err
	}
	defer osaprOwn.Close(rw)
	defer zeroBytes(sharedSecretOwn[:])

	// EncAuth for a MakeIdentity command is computed as
	//
	// encAuth = XOR(aikAuth, SHA1(sharedSecretOwn || <lastEvenNonce>))
	//
	// In this case, the last even nonce is NonceEven from OSAP for the Owner.
	xorData, err := tpmutil.Pack(sharedSecretOwn, osaprOwn.NonceEven)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(xorData)

	encAuthData := sha1.Sum(xorData)
	var encAuth Digest
	for i := range encAuth {
		encAuth[i] = aikAuth[i] ^ encAuthData[i]
	}

	var caDigest Digest
	if (pk != nil) != (label != nil) {
		return nil, errors.New("inconsistent null values between the pk and the label")
	}

	if pk != nil {
		pubKey, err := convertPubKey(pk)
		if err != nil {
			return nil, err
		}

		// We can't pack the pair of values directly, since the label is
		// included directly as bytes, without any length.
		fullpkb, err := tpmutil.Pack(pubKey)
		if err != nil {
			return nil, err
		}

		caDigestBytes := append(label, fullpkb...)
		caDigest = sha1.Sum(caDigestBytes)
	}

	rsaAIKParams := rsaKeyParams{
		KeyLength: 2048,
		NumPrimes: 2,
		//Exponent:  big.NewInt(0x10001).Bytes(), // 65537. Implicit?
	}
	packedParams, err := tpmutil.Pack(rsaAIKParams)
	if err != nil {
		return nil, err
	}

	aikParams := keyParams{
		AlgID:     AlgRSA,
		EncScheme: esNone,
		SigScheme: ssRSASaPKCS1v15SHA1,
		Params:    packedParams,
	}

	aik := &key{
		Version:         0x01010000,
		KeyUsage:        keyIdentity,
		KeyFlags:        0,
		AuthDataUsage:   authAlways,
		AlgorithmParams: aikParams,
	}

	// The digest input for MakeIdentity authentication is
	//
	// digest = SHA1(ordMakeIdentity || encAuth || caDigest || aik)
	//
	authIn := []interface{}{ordMakeIdentity, encAuth, caDigest, aik}
	ca1, err := newCommandAuth(osaprSRK.AuthHandle, osaprSRK.NonceEven, nil, sharedSecretSRK[:], authIn)
	if err != nil {
		return nil, err
	}

	ca2, err := newCommandAuth(osaprOwn.AuthHandle, osaprOwn.NonceEven, nil, sharedSecretOwn[:], authIn)
	if err != nil {
		return nil, err
	}

	k, sig, ra1, ra2, ret, err := makeIdentity(rw, encAuth, caDigest, aik, ca1, ca2)
	if err != nil {
		return nil, err
	}

	// Check response authentication.
	raIn := []interface{}{ret, ordMakeIdentity, k, tpmutil.U32Bytes(sig)}
	if err := ra1.verify(ca1.NonceOdd, sharedSecretSRK[:], raIn); err != nil {
		return nil, err
	}

	if err := ra2.verify(ca2.NonceOdd, sharedSecretOwn[:], raIn); err != nil {
		return nil, err
	}

	// TODO(tmroeder): check the signature against the pubEK.
	blob, err := tpmutil.Pack(k)
	if err != nil {
		return nil, err
	}

	return blob, nil
}

func unloadTrspiCred(blob []byte) ([]byte, error) {
	/*
	 * Trousers expects the asym blob to have an additional data in the header.
	 * The relevant data is duplicated in the TPM_SYMMETRIC_KEY struct so we parse
	 * and throw the header away.
	 * TODO(dkarch): Trousers is not doing credential activation correctly. We should
	 * remove this and instead expose the asymmetric decryption and symmetric decryption
	 * so that anyone generating a challenge for Trousers can unload the header themselves
	 * and send us a correctly formatted challenge.
	 */

	var header struct {
		Credsize  uint32
		AlgID     uint32
		EncScheme uint16
		SigScheme uint16
		Parmsize  uint32
	}

	symbuf := bytes.NewReader(blob)
	if err := binary.Read(symbuf, binary.BigEndian, &header); err != nil {
		return nil, err
	}
	// Unload the symmetric key parameters.
	parms := make([]byte, header.Parmsize)
	if err := binary.Read(symbuf, binary.BigEndian, parms); err != nil {
		return nil, err
	}
	// Unload and return the symmetrically encrypted secret.
	cred := make([]byte, header.Credsize)
	if err := binary.Read(symbuf, binary.BigEndian, cred); err != nil {
		return nil, err
	}
	return cred, nil
}

// ActivateIdentity asks the TPM to decrypt an EKPub encrypted symmetric session key
// which it uses to decrypt the symmetrically encrypted secret.
func ActivateIdentity(rw io.ReadWriter, aikAuth []byte, ownerAuth []byte, aik tpmutil.Handle, asym, sym []byte) ([]byte, error) {
	// Run OIAP for the AIK.
	oiaprAIK, err := oiap(rw)
	if err != nil {
		return nil, fmt.Errorf("failed to start OIAP session: %v", err)
	}

	// Run OSAP for the owner, reading a random OddOSAP for our initial command
	// and getting back a secret and a handle.
	sharedSecretOwn, osaprOwn, err := newOSAPSession(rw, etOwner, khOwner, ownerAuth)
	if err != nil {
		return nil, fmt.Errorf("failed to start OSAP session: %v", err)
	}
	defer osaprOwn.Close(rw)
	defer zeroBytes(sharedSecretOwn[:])

	authIn := []interface{}{ordActivateIdentity, tpmutil.U32Bytes(asym)}
	ca1, err := newCommandAuth(oiaprAIK.AuthHandle, oiaprAIK.NonceEven, nil, aikAuth, authIn)
	if err != nil {
		return nil, fmt.Errorf("newCommandAuth failed: %v", err)
	}
	ca2, err := newCommandAuth(osaprOwn.AuthHandle, osaprOwn.NonceEven, nil, sharedSecretOwn[:], authIn)
	if err != nil {
		return nil, fmt.Errorf("newCommandAuth failed: %v", err)
	}

	symkey, ra1, ra2, ret, err := activateIdentity(rw, aik, asym, ca1, ca2)
	if err != nil {
		return nil, fmt.Errorf("activateIdentity failed: %v", err)
	}

	// Check response authentication.
	raIn := []interface{}{ret, ordActivateIdentity, symkey}
	if err := ra1.verify(ca1.NonceOdd, aikAuth, raIn); err != nil {
		return nil, fmt.Errorf("aik resAuth failed to verify: %v", err)
	}

	if err := ra2.verify(ca2.NonceOdd, sharedSecretOwn[:], raIn); err != nil {
		return nil, fmt.Errorf("owner resAuth failed to verify: %v", err)
	}

	cred, err := unloadTrspiCred(sym)
	if err != nil {
		return nil, fmt.Errorf("unloadTrspiCred failed: %v", err)
	}
	var (
		block     cipher.Block
		iv        []byte
		ciphertxt []byte
		secret    []byte
	)
	switch id := symkey.AlgID; id {
	case AlgAES128:
		block, err = aes.NewCipher(symkey.Key)
		if err != nil {
			return nil, fmt.Errorf("aes.NewCipher failed: %v", err)
		}
		iv = cred[:aes.BlockSize]
		ciphertxt = cred[aes.BlockSize:]
		secret = ciphertxt
	default:
		return nil, fmt.Errorf("%v is not a supported session key algorithm", id)
	}
	switch es := symkey.EncScheme; es {
	case esSymCTR:
		stream := cipher.NewCTR(block, iv)
		stream.XORKeyStream(secret, ciphertxt)
	case esSymOFB:
		stream := cipher.NewOFB(block, iv)
		stream.XORKeyStream(secret, ciphertxt)
	case esSymCBCPKCS5:
		mode := cipher.NewCBCDecrypter(block, iv)
		mode.CryptBlocks(secret, ciphertxt)
		// Remove PKCS5 padding.
		padlen := int(secret[len(secret)-1])
		secret = secret[:len(secret)-padlen]
	default:
		return nil, fmt.Errorf("%v is not a supported encryption scheme", es)
	}

	return secret, nil
}

// ResetLockValue resets the dictionary-attack value in the TPM; this allows the
// TPM to start working again after authentication errors without waiting for
// the dictionary-attack defenses to time out. This requires owner
// authentication.
func ResetLockValue(rw io.ReadWriter, ownerAuth Digest) error {
	// Run OSAP for the Owner, reading a random OddOSAP for our initial command
	// and getting back a secret and a handle.
	sharedSecretOwn, osaprOwn, err := newOSAPSession(rw, etOwner, khOwner, ownerAuth[:])
	if err != nil {
		return err
	}
	defer osaprOwn.Close(rw)
	defer zeroBytes(sharedSecretOwn[:])

	// The digest input for ResetLockValue auth is
	//
	// digest = SHA1(ordResetLockValue)
	//
	authIn := []interface{}{ordResetLockValue}
	ca, err := newCommandAuth(osaprOwn.AuthHandle, osaprOwn.NonceEven, nil, sharedSecretOwn[:], authIn)
	if err != nil {
		return err
	}

	ra, ret, err := resetLockValue(rw, ca)
	if err != nil {
		return err
	}

	// Check response authentication.
	raIn := []interface{}{ret, ordResetLockValue}
	if err := ra.verify(ca.NonceOdd, sharedSecretOwn[:], raIn); err != nil {
		return err
	}

	return nil
}

// ownerReadInternalHelper sets up command auth and checks response auth for
// OwnerReadInternalPub. It's not exported because OwnerReadInternalPub only
// supports two fixed key handles: khEK and khSRK.
func ownerReadInternalHelper(rw io.ReadWriter, kh tpmutil.Handle, ownerAuth Digest) (*pubKey, error) {
	// Run OSAP for the Owner, reading a random OddOSAP for our initial command
	// and getting back a secret and a handle.
	sharedSecretOwn, osaprOwn, err := newOSAPSession(rw, etOwner, khOwner, ownerAuth[:])
	if err != nil {
		return nil, err
	}
	defer osaprOwn.Close(rw)
	defer zeroBytes(sharedSecretOwn[:])

	// The digest input for OwnerReadInternalPub is
	//
	// digest = SHA1(ordOwnerReadInternalPub || kh)
	//
	authIn := []interface{}{ordOwnerReadInternalPub, kh}
	ca, err := newCommandAuth(osaprOwn.AuthHandle, osaprOwn.NonceEven, nil, sharedSecretOwn[:], authIn)
	if err != nil {
		return nil, err
	}

	pk, ra, ret, err := ownerReadInternalPub(rw, kh, ca)
	if err != nil {
		return nil, err
	}

	// Check response authentication.
	raIn := []interface{}{ret, ordOwnerReadInternalPub, pk}
	if err := ra.verify(ca.NonceOdd, sharedSecretOwn[:], raIn); err != nil {
		return nil, err
	}

	return pk, nil
}

// OwnerReadSRK uses owner auth to get a blob representing the SRK.
func OwnerReadSRK(rw io.ReadWriter, ownerAuth Digest) ([]byte, error) {
	pk, err := ownerReadInternalHelper(rw, khSRK, ownerAuth)
	if err != nil {
		return nil, err
	}

	return tpmutil.Pack(pk)
}

// ReadEKCert reads the EKCert from the NVRAM.
// The TCG PC Client specifies additional headers that are to be stored with the EKCert, we parse them
// here and return only the DER encoded certificate.
// TCG PC Client Specific Implementation Specification for Conventional BIOS 7.4.4
// https://www.trustedcomputinggroup.org/wp-content/uploads/TCG_PCClientImplementation_1-21_1_00.pdf
func ReadEKCert(rw io.ReadWriter, ownAuth Digest) ([]byte, error) {
	const (
		certIndex                 = 0x1000f000 // TPM_NV_INDEX_EKCert (TPM Main Part 2 TPM Structures 19.1.2)
		certTagPCClientStoredCert = 0x1001     // TCG_TAG_PCCLIENT_STORED_CERT
		certTagPCClientFullCert   = 0x1002     // TCG_TAG_PCCLIENT_FULL_CERT
		tcgFullCert               = 0          // TCG_FULL_CERT
		tcgPartialSmallCert       = 1          // TCG_PARTIAL_SMALL_CERT
	)
	offset := uint32(0)
	var header struct {
		Tag      uint16
		CertType uint8
		CertSize uint16
	}

	data, err := NVReadValue(rw, certIndex, offset, uint32(binary.Size(header)), []byte(ownAuth[:]))
	if err != nil {
		return nil, err
	}
	offset = offset + uint32(binary.Size(header))
	buff := bytes.NewReader(data)

	if err := binary.Read(buff, binary.BigEndian, &header); err != nil {
		return nil, err
	}

	if header.Tag != certTagPCClientStoredCert {
		return nil, fmt.Errorf("invalid certificate")
	}

	var bufSize uint32
	switch header.CertType {
	case tcgFullCert:
		var tag uint16
		data, err := NVReadValue(rw, certIndex, offset, uint32(binary.Size(tag)), []byte(ownAuth[:]))
		if err != nil {
			return nil, err
		}
		bufSize = uint32(header.CertSize) + offset
		offset = offset + uint32(binary.Size(tag))
		buff = bytes.NewReader(data)

		if err := binary.Read(buff, binary.BigEndian, &tag); err != nil {
			return nil, err
		}

		if tag != certTagPCClientFullCert {
			return nil, fmt.Errorf("certificate type and tag do not match")
		}
	case tcgPartialSmallCert:
		return nil, fmt.Errorf("certType is not TCG_FULL_CERT: currently do not support partial certs")
	default:
		return nil, fmt.Errorf("invalid certType: 0x%x", header.CertType)
	}

	var ekbuf []byte
	for offset < bufSize {
		length := bufSize - offset
		// TPMs can only read so much memory per command so we read in 128byte chunks.
		// 128 was taken from go-tspi. The actual max read seems to be platform dependent
		// but cannot be queried on TPM1.2 (and does not seem to appear in any documentation).
		if length > 128 {
			length = 128
		}
		data, err = NVReadValue(rw, certIndex, offset, length, []byte(ownAuth[:]))
		if err != nil {
			return nil, err
		}

		ekbuf = append(ekbuf, data...)
		offset += length
	}

	return ekbuf, nil
}

// NVDefineSpace implements the reservation of NVRAM as specified in:
// TPM-Main-Part-3-Commands_v1.2_rev116_01032011, P. 212
func NVDefineSpace(rw io.ReadWriter, nvData NVDataPublic, ownAuth []byte) error {
	var ra *responseAuth
	var ret uint32
	if ownAuth == nil {
	} else {
		sharedSecretOwn, osaprOwn, err := newOSAPSession(rw, etOwner, khOwner, ownAuth[:])
		if err != nil {
			return fmt.Errorf("failed to start new auth session: %v", err)
		}
		defer osaprOwn.Close(rw)
		defer zeroBytes(sharedSecretOwn[:])

		// encAuth: NV_Define_Space is a special case where no encryption is used.
		// See spec: TPM-Main-Part-1-Design-Principles_v1.2_rev116_01032011, P. 81
		xorData, err := tpmutil.Pack(sharedSecretOwn, osaprOwn.NonceEven)
		if err != nil {
			return err
		}
		defer zeroBytes(xorData)

		encAuthData := sha1.Sum(xorData)

		authIn := []interface{}{ordNVDefineSpace, nvData, encAuthData}
		ca, err := newCommandAuth(osaprOwn.AuthHandle, osaprOwn.NonceEven, nil, sharedSecretOwn[:], authIn)
		if err != nil {
			return err
		}
		ra, ret, err = nvDefineSpace(rw, nvData, encAuthData, ca)
		if err != nil {
			return fmt.Errorf("failed to define space in NVRAM: %v", err)
		}
		raIn := []interface{}{ret, ordNVDefineSpace}
		if err := ra.verify(ca.NonceOdd, sharedSecretOwn[:], raIn); err != nil {
			return fmt.Errorf("failed to verify authenticity of response: %v", err)
		}
	}
	return nil
}

// NVReadValue returns the value from a given index, offset, and length in NVRAM.
// See TPM-Main-Part-2-TPM-Structures 19.1.
// If TPM isn't locked, no authentication is needed.
// This is for platform suppliers only.
// See TPM-Main-Part-3-Commands-20.4
func NVReadValue(rw io.ReadWriter, index, offset, len uint32, ownAuth []byte) ([]byte, error) {
	if ownAuth == nil {
		data, _, _, err := nvReadValue(rw, index, offset, len, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to read from NVRAM: %v", err)
		}
		return data, nil
	}
	sharedSecretOwn, osaprOwn, err := newOSAPSession(rw, etOwner, khOwner, ownAuth[:])
	if err != nil {
		return nil, fmt.Errorf("failed to start new auth session: %v", err)
	}
	defer osaprOwn.Close(rw)
	defer zeroBytes(sharedSecretOwn[:])
	authIn := []interface{}{ordNVReadValue, index, offset, len}
	ca, err := newCommandAuth(osaprOwn.AuthHandle, osaprOwn.NonceEven, nil, sharedSecretOwn[:], authIn)
	if err != nil {
		return nil, fmt.Errorf("failed to construct owner auth fields: %v", err)
	}
	data, ra, ret, err := nvReadValue(rw, index, offset, len, ca)
	if err != nil {
		return nil, fmt.Errorf("failed to read from NVRAM: %v", err)
	}
	raIn := []interface{}{ret, ordNVReadValue, tpmutil.U32Bytes(data)}
	if err := ra.verify(ca.NonceOdd, sharedSecretOwn[:], raIn); err != nil {
		return nil, fmt.Errorf("failed to verify authenticity of response: %v", err)
	}

	return data, nil
}

// NVReadValueAuth returns the value from a given index, offset, and length in NVRAM.
// See TPM-Main-Part-2-TPM-Structures 19.1.
// If TPM is locked, authentication is mandatory.
// See TPM-Main-Part-3-Commands-20.5
func NVReadValueAuth(rw io.ReadWriter, index, offset, len uint32, auth []byte) ([]byte, error) {
	if auth == nil {
		return nil, fmt.Errorf("no auth value given but mandatory")
	}
	sharedSecret, osapr, err := newOSAPSession(rw, etOwner, khOwner, auth[:])
	if err != nil {
		return nil, fmt.Errorf("failed to start new auth session: %v", err)
	}
	defer osapr.Close(rw)
	defer zeroBytes(sharedSecret[:])
	authIn := []interface{}{ordNVReadValueAuth, index, offset, len}
	ca, err := newCommandAuth(osapr.AuthHandle, osapr.NonceEven, nil, sharedSecret[:], authIn)
	if err != nil {
		return nil, fmt.Errorf("failed to construct auth fields: %v", err)
	}
	data, ra, ret, err := nvReadValue(rw, index, offset, len, ca)
	if err != nil {
		return nil, fmt.Errorf("failed to read from NVRAM: %v", err)
	}
	raIn := []interface{}{ret, ordNVReadValueAuth, tpmutil.U32Bytes(data)}
	if err := ra.verify(ca.NonceOdd, sharedSecret[:], raIn); err != nil {
		return nil, fmt.Errorf("failed to verify authenticity of response: %v", err)
	}

	return data, nil
}

// NVWriteValue for writing to the NVRAM. Needs a index for a defined space in NVRAM.
// See TPM-Main-Part-3-Commands_v1.2_rev116_01032011, P216
func NVWriteValue(rw io.ReadWriter, index, offset uint32, data []byte, ownAuth []byte) error {
	if ownAuth == nil {
		if _, _, _, err := nvWriteValue(rw, index, offset, uint32(len(data)), data, nil); err != nil {
			return fmt.Errorf("failed to write to NVRAM: %v", err)
		}
		return nil
	}
	sharedSecretOwn, osaprOwn, err := newOSAPSession(rw, etOwner, khOwner, ownAuth[:])
	if err != nil {
		return fmt.Errorf("failed to start new auth session: %v", err)
	}
	defer osaprOwn.Close(rw)
	defer zeroBytes(sharedSecretOwn[:])
	authIn := []interface{}{ordNVWriteValue, index, offset, len(data), data}
	ca, err := newCommandAuth(osaprOwn.AuthHandle, osaprOwn.NonceEven, nil, sharedSecretOwn[:], authIn)
	if err != nil {
		return fmt.Errorf("failed to construct owner auth fields: %v", err)
	}
	data, ra, ret, err := nvWriteValue(rw, index, offset, uint32(len(data)), data, ca)
	if err != nil {
		return fmt.Errorf("failed to write to NVRAM: %v", err)
	}
	raIn := []interface{}{ret, ordNVWriteValue, tpmutil.U32Bytes(data)}
	if err := ra.verify(ca.NonceOdd, sharedSecretOwn[:], raIn); err != nil {
		return fmt.Errorf("failed to verify authenticity of response: %v", err)
	}
	return nil
}

// NVWriteValueAuth for authenticated writing to the NVRAM.
// Needs a index of a defined space in NVRAM.
// See TPM-Main-Part-2-TPM-Structures 19.1.
// If TPM is locked, authentification is mandatory.
// See TPM-Main-Part-3-Commands_v1.2_rev116_01032011, P216
func NVWriteValueAuth(rw io.ReadWriter, index, offset uint32, data []byte, auth []byte) error {
	if auth == nil {
		return fmt.Errorf("no auth value given but mandatory")
	}
	sharedSecret, osapr, err := newOSAPSession(rw, etOwner, khOwner, auth[:])
	if err != nil {
		return fmt.Errorf("failed to start new auth session: %v", err)
	}
	defer osapr.Close(rw)
	defer zeroBytes(sharedSecret[:])
	authIn := []interface{}{ordNVWriteValueAuth, index, offset, len(data), data}
	ca, err := newCommandAuth(osapr.AuthHandle, osapr.NonceEven, nil, sharedSecret[:], authIn)
	if err != nil {
		return fmt.Errorf("failed to construct auth fields: %v", err)
	}
	data, ra, ret, err := nvWriteValue(rw, index, offset, uint32(len(data)), data, ca)
	if err != nil {
		return fmt.Errorf("failed to write to NVRAM: %v", err)
	}
	raIn := []interface{}{ret, ordNVWriteValueAuth, tpmutil.U32Bytes(data)}
	if err := ra.verify(ca.NonceOdd, sharedSecret[:], raIn); err != nil {
		return fmt.Errorf("failed to verify authenticity of response: %v", err)
	}
	return nil
}

// OwnerReadPubEK uses owner auth to get a blob representing the public part of the
// endorsement key.
func OwnerReadPubEK(rw io.ReadWriter, ownerAuth Digest) ([]byte, error) {
	pk, err := ownerReadInternalHelper(rw, khEK, ownerAuth)
	if err != nil {
		return nil, err
	}

	return tpmutil.Pack(pk)
}

// ReadPubEK reads the public part of the endorsement key when no owner is
// established.
func ReadPubEK(rw io.ReadWriter) ([]byte, error) {
	var n Nonce
	if _, err := rand.Read(n[:]); err != nil {
		return nil, err
	}

	pk, d, _, err := readPubEK(rw, n)
	if err != nil {
		return nil, err
	}

	// Recompute the hash of the pk and the nonce to defend against replay
	// attacks.
	b, err := tpmutil.Pack(pk, n)
	if err != nil {
		return nil, err
	}

	s := sha1.Sum(b)
	// There's no need for constant-time comparison of these hash values,
	// since no secret is involved.
	if !bytes.Equal(s[:], d[:]) {
		return nil, errors.New("the ReadPubEK operation failed the replay check")
	}

	return tpmutil.Pack(pk)
}

// GetManufacturer returns the manufacturer ID
func GetManufacturer(rw io.ReadWriter) ([]byte, error) {
	return getCapability(rw, CapProperty, SubCapPropManufacturer)
}

// GetPermanentFlags returns the TPM_PERMANENT_FLAGS structure.
func GetPermanentFlags(rw io.ReadWriter) (PermanentFlags, error) {
	var ret PermanentFlags

	raw, err := getCapability(rw, CapFlag, SubCapFlagPermanent)
	if err != nil {
		return ret, err
	}

	_, err = tpmutil.Unpack(raw, &ret)
	return ret, err
}

// GetAlgs returns a list of algorithms supported by the TPM device.
func GetAlgs(rw io.ReadWriter) ([]Algorithm, error) {
	var algs []Algorithm
	for i := AlgRSA; i <= AlgXOR; i++ {
		buf, err := getCapability(rw, CapAlg, uint32(i))
		if err != nil {
			return nil, err
		}
		if uint8(buf[0]) > 0 {
			algs = append(algs, Algorithm(i))
		}

	}
	return algs, nil
}

// GetCapVersionVal returns the decoded contents of TPM_CAP_VERSION_INFO.
func GetCapVersionVal(rw io.ReadWriter) (*CapVersionInfo, error) {
	var capVer CapVersionInfo
	buf, err := getCapability(rw, CapVersion, 0)
	if err != nil {
		return nil, err
	}
	if err := capVer.Decode(buf); err != nil {
		return nil, err
	}
	return &capVer, nil
}

// GetNVList returns a list of TPM_NV_INDEX values that
// are currently allocated NV storage through TPM_NV_DefineSpace.
func GetNVList(rw io.ReadWriter) ([]uint32, error) {
	buf, err := getCapability(rw, CapNVList, 0)
	if err != nil {
		return nil, err
	}
	nvList := make([]uint32, len(buf)/4)
	for i := range nvList {
		nvList[i] = uint32(binary.BigEndian.Uint32(buf[i*4 : (i+1)*4]))
	}

	return nvList, err
}

// GetNVIndex returns the structure of NVDataPublic which contains
// information about the requested NV Index.
// See: TPM-Main-Part-2-TPM-Structures_v1.2_rev116_01032011, P.167
func GetNVIndex(rw io.ReadWriter, nvIndex uint32) (*NVDataPublic, error) {
	var nvInfo NVDataPublic
	buf, _ := getCapability(rw, CapNVIndex, nvIndex)
	if _, err := tpmutil.Unpack(buf, &nvInfo); err != nil {
		return &nvInfo, err
	}
	return &nvInfo, nil
}

// GetCapabilityRaw reads the requested capability and sub-capability from the
// TPM and returns it as a []byte. Where possible, prefer the convenience
// functions above, which return higher-level structs for easier handling.
func GetCapabilityRaw(rw io.ReadWriter, cap, subcap uint32) ([]byte, error) {
	return getCapability(rw, cap, subcap)
}

// OwnerClear uses owner auth to clear the TPM. After this operation, the TPM
// can change ownership.
func OwnerClear(rw io.ReadWriter, ownerAuth Digest) error {
	// Run OSAP for the Owner, reading a random OddOSAP for our initial command
	// and getting back a secret and a handle.
	sharedSecretOwn, osaprOwn, err := newOSAPSession(rw, etOwner, khOwner, ownerAuth[:])
	if err != nil {
		return err
	}
	defer osaprOwn.Close(rw)
	defer zeroBytes(sharedSecretOwn[:])

	// The digest input for OwnerClear is
	//
	// digest = SHA1(ordOwnerClear)
	//
	authIn := []interface{}{ordOwnerClear}
	ca, err := newCommandAuth(osaprOwn.AuthHandle, osaprOwn.NonceEven, nil, sharedSecretOwn[:], authIn)
	if err != nil {
		return err
	}

	ra, ret, err := ownerClear(rw, ca)
	if err != nil {
		return err
	}

	// Check response authentication.
	raIn := []interface{}{ret, ordOwnerClear}
	if err := ra.verify(ca.NonceOdd, sharedSecretOwn[:], raIn); err != nil {
		return err
	}

	return nil
}

// TakeOwnership takes over a TPM and inserts a new owner auth value and
// generates a new SRK, associating it with a new SRK auth value. This
// operation can only be performed if there isn't already an owner for the TPM.
// The pub EK blob can be acquired by calling ReadPubEK if there is no owner, or
// OwnerReadPubEK if there is.
func TakeOwnership(rw io.ReadWriter, newOwnerAuth Digest, newSRKAuth Digest, pubEK []byte) error {

	// Encrypt the owner and SRK auth with the endorsement key.
	ek, err := UnmarshalPubRSAPublicKey(pubEK)
	if err != nil {
		return err
	}
	encOwnerAuth, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, ek, newOwnerAuth[:], oaepLabel)
	if err != nil {
		return err
	}
	encSRKAuth, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, ek, newSRKAuth[:], oaepLabel)
	if err != nil {
		return err
	}

	// The params for the SRK have very tight requirements:
	// - KeyLength must be 2048
	// - alg must be RSA
	// - Enc must be OAEP SHA1 MGF1
	// - Sig must be None
	// - Key usage must be Storage
	// - Key must not be migratable
	srkRSAParams := rsaKeyParams{
		KeyLength: 2048,
		NumPrimes: 2,
	}
	srkpb, err := tpmutil.Pack(srkRSAParams)
	if err != nil {
		return err
	}
	srkParams := keyParams{
		AlgID:     AlgRSA,
		EncScheme: esRSAEsOAEPSHA1MGF1,
		SigScheme: ssNone,
		Params:    srkpb,
	}
	srk := &key{
		Version:         0x01010000,
		KeyUsage:        keyStorage,
		KeyFlags:        0,
		AuthDataUsage:   authAlways,
		AlgorithmParams: srkParams,
	}

	// Get command auth using OIAP with the new owner auth.
	oiapr, err := oiap(rw)
	if err != nil {
		return err
	}
	defer oiapr.Close(rw)

	// The digest for TakeOwnership is
	//
	// SHA1(ordTakeOwnership || pidOwner || encOwnerAuth || encSRKAuth || srk)
	authIn := []interface{}{ordTakeOwnership, pidOwner, tpmutil.U32Bytes(encOwnerAuth), tpmutil.U32Bytes(encSRKAuth), srk}
	ca, err := newCommandAuth(oiapr.AuthHandle, oiapr.NonceEven, nil, newOwnerAuth[:], authIn)
	if err != nil {
		return err
	}

	k, ra, ret, err := takeOwnership(rw, encOwnerAuth, encSRKAuth, srk, ca)
	if err != nil {
		return err
	}

	raIn := []interface{}{ret, ordTakeOwnership, k}
	return ra.verify(ca.NonceOdd, newOwnerAuth[:], raIn)
}

func createWrapKeyHelper(rw io.ReadWriter, srkAuth []byte, keyFlags KeyFlags, usageAuth Digest, migrationAuth Digest, pcrs []int) (*key, error) {
	// Run OSAP for the SRK, reading a random OddOSAP for our initial
	// command and getting back a secret and a handle.
	sharedSecret, osapr, err := newOSAPSession(rw, etSRK, khSRK, srkAuth)
	if err != nil {
		return nil, err
	}
	defer osapr.Close(rw)
	defer zeroBytes(sharedSecret[:])

	xorData, err := tpmutil.Pack(sharedSecret, osapr.NonceEven)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(xorData)

	// We have to come up with NonceOdd early to encrypt the migration auth.
	var nonceOdd Nonce
	if _, err := rand.Read(nonceOdd[:]); err != nil {
		return nil, err
	}

	// ADIP (Authorization Data Insertion Protocol) is based on NonceEven for the first auth value
	// encrypted by the protocol, and NonceOdd for the second auth value. This is so that the two
	// keystreams are independent - otherwise, an eavesdropping attacker could XOR the two encrypted
	// values together to cancel out the key and calculate (usageAuth ^ migrationAuth).
	xorData2, err := tpmutil.Pack(sharedSecret, nonceOdd)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(xorData2)

	encAuthDataKey := sha1.Sum(xorData)
	defer zeroBytes(encAuthDataKey[:])
	encAuthDataKey2 := sha1.Sum(xorData2)
	defer zeroBytes(encAuthDataKey2[:])

	var encUsageAuth Digest
	for i := range usageAuth {
		encUsageAuth[i] = encAuthDataKey[i] ^ usageAuth[i]
	}
	var encMigrationAuth Digest
	for i := range migrationAuth {
		encMigrationAuth[i] = encAuthDataKey2[i] ^ migrationAuth[i]
	}

	rParams := rsaKeyParams{
		KeyLength: 2048,
		NumPrimes: 2,
	}
	rParamsPacked, err := tpmutil.Pack(&rParams)
	if err != nil {
		return nil, err
	}

	var pcrInfoBytes []byte
	if len(pcrs) > 0 {
		pcrInfo, err := newPCRInfo(rw, pcrs)
		if err != nil {
			return nil, err
		}
		pcrInfoBytes, err = tpmutil.Pack(pcrInfo)
		if err != nil {
			return nil, err
		}
	}

	keyInfo := &key{
		Version:       0x01010000,
		KeyUsage:      keySigning,
		KeyFlags:      keyFlags,
		AuthDataUsage: authAlways,
		AlgorithmParams: keyParams{
			AlgID:     AlgRSA,
			EncScheme: esNone,
			SigScheme: ssRSASaPKCS1v15DER,
			Params:    rParamsPacked,
		},
		PCRInfo: pcrInfoBytes,
	}

	authIn := []interface{}{ordCreateWrapKey, encUsageAuth, encMigrationAuth, keyInfo}
	ca, err := newCommandAuth(osapr.AuthHandle, osapr.NonceEven, &nonceOdd, sharedSecret[:], authIn)
	if err != nil {
		return nil, err
	}

	k, ra, ret, err := createWrapKey(rw, encUsageAuth, encMigrationAuth, keyInfo, ca)
	if err != nil {
		return nil, err
	}

	raIn := []interface{}{ret, ordCreateWrapKey, k}
	if err = ra.verify(ca.NonceOdd, sharedSecret[:], raIn); err != nil {
		return nil, err
	}

	return k, nil
}

// CreateWrapKey creates a new RSA key for signatures inside the TPM. It is
// wrapped by the SRK (which is to say, the SRK is the parent key). The key can
// be bound to the specified PCR numbers so that it can only be used for
// signing if the PCR values of those registers match. The pcrs parameter can
// be nil in which case the key is not bound to any PCRs. The usageAuth
// parameter defines the auth key for using this new key. The migrationAuth
// parameter would be used for authorizing migration of the key (although this
// code currently disables migration).
func CreateWrapKey(rw io.ReadWriter, srkAuth []byte, usageAuth Digest, migrationAuth Digest, pcrs []int) ([]byte, error) {
	k, err := createWrapKeyHelper(rw, srkAuth, 0, usageAuth, migrationAuth, pcrs)
	if err != nil {
		return nil, err
	}
	keyblob, err := tpmutil.Pack(k)
	if err != nil {
		return nil, err
	}
	return keyblob, nil
}

// CreateMigratableWrapKey creates a new RSA key as in CreateWrapKey, but the
// key is migratable (with the given migration auth).
// Returns the loadable KeyBlob as well as just the encrypted private part, for
// migration.
func CreateMigratableWrapKey(rw io.ReadWriter, srkAuth []byte, usageAuth Digest, migrationAuth Digest, pcrs []int) ([]byte, []byte, error) {
	k, err := createWrapKeyHelper(rw, srkAuth, keyMigratable, usageAuth, migrationAuth, pcrs)
	if err != nil {
		return nil, nil, err
	}
	keyblob, err := tpmutil.Pack(k)
	if err != nil {
		return nil, nil, err
	}
	return keyblob, k.EncData, nil
}

// AuthorizeMigrationKey authorizes a given public key for use in migrating
// migratable keys. The scheme is REWRAP.
func AuthorizeMigrationKey(rw io.ReadWriter, ownerAuth Digest, migrationKey crypto.PublicKey) ([]byte, error) {
	// Run OSAP for the OwnerAuth, reading a random OddOSAP for our initial
	// command and getting back a secret and a handle.
	sharedSecret, osapr, err := newOSAPSession(rw, etOwner, khOwner, ownerAuth[:])
	if err != nil {
		return nil, err
	}
	defer osapr.Close(rw)
	defer zeroBytes(sharedSecret[:])

	var pub *pubKey
	if migrationKey != nil {
		pub, err = convertPubKey(migrationKey)
		if err != nil {
			return nil, err
		}
		// convertPubKey is designed for signing keys.
		pub.AlgorithmParams.EncScheme = esRSAEsOAEPSHA1MGF1
		pub.AlgorithmParams.SigScheme = ssNone
		rsaParams := rsaKeyParams{
			KeyLength: 2048,
			NumPrimes: 2,
			//Exponent: default (omit)
		}
		pub.AlgorithmParams.Params, err = tpmutil.Pack(rsaParams)
		if err != nil {
			return nil, err
		}
	}

	scheme := msRewrap

	// The digest for auth for the authorizeMigrationKey command is computed as
	// SHA1(ordAuthorizeMigrationkey || migrationScheme || migrationKey)
	authIn := []interface{}{ordAuthorizeMigrationKey, scheme, pub}

	// The commandAuth uses the shared secret as an HMAC key.
	ca, err := newCommandAuth(osapr.AuthHandle, osapr.NonceEven, nil, sharedSecret[:], authIn)
	if err != nil {
		return nil, err
	}

	migrationAuth, _, _, err := authorizeMigrationKey(rw, scheme, *pub, ca)
	if err != nil {
		return nil, err
	}

	// For now, ignore the response authentication.
	return migrationAuth, nil
}

// CreateMigrationBlob performs a Rewrap migration of the given key blob.
func CreateMigrationBlob(rw io.ReadWriter, srkAuth Digest, migrationAuth Digest, keyBlob []byte, migrationKeyBlob []byte) ([]byte, error) {
	// Run OSAP for the SRK, reading a random OddOSAP for our initial
	// command and getting back a secret and a handle.
	sharedSecret, osapr, err := newOSAPSession(rw, etSRK, khSRK, srkAuth[:])
	if err != nil {
		return nil, err
	}
	defer osapr.Close(rw)
	defer zeroBytes(sharedSecret[:])

	// The createMigrationBlob command needs an OIAP session in addition to the
	// OSAP session.
	oiapr, err := oiap(rw)
	if err != nil {
		return nil, err
	}
	defer oiapr.Close(rw)

	encData := tpmutil.U32Bytes(keyBlob)

	// The digest for auth1 and auth2 for the createMigrationBlob command is
	// SHA1(ordCreateMigrationBlob || migrationScheme || migrationKeyBlob || encData)
	authIn := []interface{}{ordCreateMigrationBlob, msRewrap, migrationKeyBlob, encData}

	// The first commandAuth uses the shared secret as an HMAC key.
	ca1, err := newCommandAuth(osapr.AuthHandle, osapr.NonceEven, nil, sharedSecret[:], authIn)
	if err != nil {
		return nil, err
	}

	// The second commandAuth is based on OIAP instead of OSAP and uses the
	// migration auth as the HMAC key.
	ca2, err := newCommandAuth(oiapr.AuthHandle, oiapr.NonceEven, nil, migrationAuth[:], authIn)
	if err != nil {
		return nil, err
	}

	_, outData, _, _, _, err := createMigrationBlob(rw, khSRK, msRewrap, migrationKeyBlob, encData, ca1, ca2)
	if err != nil {
		return nil, err
	}

	// For now, ignore the response authenticatino.
	return outData, nil
}

// https://golang.org/src/crypto/rsa/pkcs1v15.go?s=8762:8862#L204
var hashPrefixes = map[crypto.Hash][]byte{
	crypto.MD5:       {0x30, 0x20, 0x30, 0x0c, 0x06, 0x08, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0x0d, 0x02, 0x05, 0x05, 0x00, 0x04, 0x10},
	crypto.SHA1:      {0x30, 0x21, 0x30, 0x09, 0x06, 0x05, 0x2b, 0x0e, 0x03, 0x02, 0x1a, 0x05, 0x00, 0x04, 0x14},
	crypto.SHA224:    {0x30, 0x2d, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x02, 0x04, 0x05, 0x00, 0x04, 0x1c},
	crypto.SHA256:    {0x30, 0x31, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x02, 0x01, 0x05, 0x00, 0x04, 0x20},
	crypto.SHA384:    {0x30, 0x41, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x02, 0x02, 0x05, 0x00, 0x04, 0x30},
	crypto.SHA512:    {0x30, 0x51, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x02, 0x03, 0x05, 0x00, 0x04, 0x40},
	crypto.MD5SHA1:   {}, // A special TLS case which doesn't use an ASN1 prefix.
	crypto.RIPEMD160: {0x30, 0x20, 0x30, 0x08, 0x06, 0x06, 0x28, 0xcf, 0x06, 0x03, 0x00, 0x31, 0x04, 0x14},
}

// Sign will sign a digest using the supplied key handle. Uses PKCS1v15 signing, which means the hash OID is prefixed to the
// hash before it is signed. Therefore the hash used needs to be passed as the hash parameter to determine the right
// prefix.
func Sign(rw io.ReadWriter, keyAuth []byte, keyHandle tpmutil.Handle, hash crypto.Hash, hashed []byte) ([]byte, error) {
	prefix, ok := hashPrefixes[hash]
	if !ok {
		return nil, errors.New("Unsupported hash")
	}
	data := append(prefix, hashed...)

	// Run OSAP for the SRK, reading a random OddOSAP for our initial
	// command and getting back a secret and a handle.
	sharedSecret, osapr, err := newOSAPSession(rw, etKeyHandle, keyHandle, keyAuth)
	if err != nil {
		return nil, err
	}
	defer osapr.Close(rw)
	defer zeroBytes(sharedSecret[:])

	authIn := []interface{}{ordSign, tpmutil.U32Bytes(data)}
	ca, err := newCommandAuth(osapr.AuthHandle, osapr.NonceEven, nil, sharedSecret[:], authIn)
	if err != nil {
		return nil, err
	}

	signature, ra, ret, err := sign(rw, keyHandle, data, ca)
	if err != nil {
		return nil, err
	}

	raIn := []interface{}{ret, ordSign, tpmutil.U32Bytes(signature)}
	err = ra.verify(ca.NonceOdd, sharedSecret[:], raIn)
	if err != nil {
		return nil, err
	}

	return signature, nil
}

// PcrReset resets the given PCRs. Given typical locality restrictions, this can usually only be 16 or 23.
func PcrReset(rw io.ReadWriter, pcrs []int) error {
	pcrSelect, err := newPCRSelection(pcrs)
	if err != nil {
		return err
	}
	err = pcrReset(rw, pcrSelect)
	if err != nil {
		return err
	}
	return nil
}

// ForceClear is normally used by firmware but on some platforms
// vendors got it wrong and didn't call TPM_DisableForceClear.
// It removes forcefully the ownership of the TPM.
func ForceClear(rw io.ReadWriter) error {
	in := []interface{}{}
	out := []interface{}{}
	_, err := submitTPMRequest(rw, tagRQUCommand, ordForceClear, in, out)

	return err
}

// Startup performs TPM_Startup(TPM_ST_CLEAR) to initialize the TPM.
func startup(rw io.ReadWriter) error {
	var typ uint16 = 0x0001 // TPM_ST_CLEAR
	in := []interface{}{typ}
	out := []interface{}{}
	_, err := submitTPMRequest(rw, tagRQUCommand, ordStartup, in, out)

	return err
}

// createEK performs TPM_CreateEndorsementKeyPair to create the EK in the TPM.
func createEK(rw io.ReadWriter) error {
	antiReplay := Nonce{}
	keyInfo := []byte{
		0x00, 0x00, 0x00, 0x01, // Algorithm = RSA
		0x00, 0x03, // EncScheme = OAEP
		0x00, 0x01, // SigScheme = None
		0x00, 0x00, 0x00, 0x0c, // ParamsSize = 12
		0x00, 0x00, 0x08, 0x00, // KeyLength = 2048
		0x00, 0x00, 0x00, 0x02, // NumPrimes = 2
		0x00, 0x00, 0x00, 0x00, // ExponentSize = 0 (default 65537 exponent)
	}
	in := []interface{}{antiReplay, keyInfo}
	out := []interface{}{}
	_, err := submitTPMRequest(rw, tagRQUCommand, ordCreateEndorsementKeyPair, in, out)

	return err
}
