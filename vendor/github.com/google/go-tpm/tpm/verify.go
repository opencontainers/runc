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
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"errors"
	"math/big"

	"github.com/google/go-tpm/tpmutil"
)

// This file provides functions to extract a crypto/rsa public key from a key
// blob or a TPM_KEY of the right type. It also provides a function for
// verifying a quote value given a public key for the key it was signed with.

// UnmarshalRSAPublicKey takes in a blob containing a serialized RSA TPM_KEY and
// converts it to a crypto/rsa.PublicKey.
func UnmarshalRSAPublicKey(keyBlob []byte) (*rsa.PublicKey, error) {
	// Parse the blob as a key.
	var k key
	if _, err := tpmutil.Unpack(keyBlob, &k); err != nil {
		return nil, err
	}

	return k.unmarshalRSAPublicKey()
}

// unmarshalRSAPublicKey unmarshals a TPM key into a crypto/rsa.PublicKey.
func (k *key) unmarshalRSAPublicKey() (*rsa.PublicKey, error) {
	// Currently, we only support algRSA
	if k.AlgorithmParams.AlgID != AlgRSA {
		return nil, errors.New("only TPM_ALG_RSA is supported")
	}

	// This means that k.AlgorithmsParams.Params is an rsaKeyParams, which is
	// enough to create the exponent, and k.PubKey contains the key.
	var rsakp rsaKeyParams
	if _, err := tpmutil.Unpack(k.AlgorithmParams.Params, &rsakp); err != nil {
		return nil, err
	}

	// Make sure that the exponent will fit into an int before using it blindly.
	if len(rsakp.Exponent) > 4 {
		return nil, errors.New("exponent value doesn't fit into an int")
	}
	pk := &rsa.PublicKey{
		N: new(big.Int).SetBytes(k.PubKey),
		// The exponent isn't set here, but it's fixed to 0x10001
		E: 0x10001,
	}
	return pk, nil
}

// UnmarshalPubRSAPublicKey takes in a blob containing a serialized RSA
// TPM_PUBKEY and converts it to a crypto/rsa.PublicKey.
func UnmarshalPubRSAPublicKey(keyBlob []byte) (*rsa.PublicKey, error) {
	// Parse the blob as a key.
	var pk pubKey
	if _, err := tpmutil.Unpack(keyBlob, &pk); err != nil {
		return nil, err
	}

	return pk.unmarshalRSAPublicKey()
}

// unmarshalRSAPublicKey unmarshals a TPM pub key into a crypto/rsa.PublicKey.
// This is almost identical to the identically named function for a TPM key.
func (pk *pubKey) unmarshalRSAPublicKey() (*rsa.PublicKey, error) {
	// Currently, we only support AlgRSA
	if pk.AlgorithmParams.AlgID != AlgRSA {
		return nil, errors.New("only TPM_ALG_RSA is supported")
	}

	// This means that pk.AlgorithmsParams.Params is an rsaKeyParams, which is
	// enough to create the exponent, and pk.Key contains the key.
	var rsakp rsaKeyParams
	if _, err := tpmutil.Unpack(pk.AlgorithmParams.Params, &rsakp); err != nil {
		return nil, err
	}

	// Make sure that the exponent will fit into an int before using it blindly.
	if len(rsakp.Exponent) > 4 {
		return nil, errors.New("exponent value doesn't fit into an int")
	}
	rsapk := &rsa.PublicKey{
		N: new(big.Int).SetBytes(pk.Key),
		// The exponent isn't set here, but it's fixed to 0x10001
		E: 0x10001,
	}
	return rsapk, nil
}

// NewQuoteInfo computes a quoteInfo structure for a given pair of data and PCR
// values.
func NewQuoteInfo(data []byte, pcrNums []int, pcrs []byte) ([]byte, error) {
	// Compute the composite hash for these PCRs.
	pcrSel, err := newPCRSelection(pcrNums)
	if err != nil {
		return nil, err
	}

	comp, err := createPCRComposite(pcrSel.Mask, pcrs)
	if err != nil {
		return nil, err
	}

	qi := &quoteInfo{
		Version: quoteVersion,
		Fixed:   fixedQuote,
		Nonce:   sha1.Sum(data),
	}
	copy(qi.CompositeDigest[:], comp)

	return tpmutil.Pack(qi)
}

// VerifyQuote verifies a quote against a given set of PCRs.
func VerifyQuote(pk *rsa.PublicKey, data []byte, quote []byte, pcrNums []int, pcrs []byte) error {
	p, err := NewQuoteInfo(data, pcrNums, pcrs)
	if err != nil {
		return err
	}

	s := sha1.Sum(p)

	// Try to do a direct encryption to reverse the value and see if it's padded
	// with PKCS1v1.5.
	return rsa.VerifyPKCS1v15(pk, crypto.SHA1, s[:], quote)
}

// TODO(tmroeder): add VerifyQuote2 instead of VerifyQuote. This means I'll
// probably have to look at the signature scheme and use that to choose how to
// verify the signature, whether PKCS1v1.5 or OAEP. And this will have to be set
// on the key before it's passed to ordQuote2
// TODO(tmroeder): handle key12
