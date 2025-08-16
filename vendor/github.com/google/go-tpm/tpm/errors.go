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
	"strconv"
)

// A tpmError is an error value from the TPM.
type tpmError uint32

// Error produces a string for the given TPM Error code
func (o tpmError) Error() string {
	if s, ok := tpmErrMsgs[o]; ok {
		return "tpm: " + s
	}

	return "tpm: unknown error code " + strconv.Itoa(int(o))
}

// These are the TPM error codes from the spec.
const (
	_                    = iota
	errAuthFail tpmError = iota
	errBadIndex
	errBadParameter
	errAuditFailure
	errClearDisabled
	errDeactivated
	errDisabled
	errDisabledCmd
	errFail
	errBadOrdinal
	errInstallDisabled
	errInvalidKeyHandle
	errKeyNotFound
	errInappropriateEnc
	errMigrateFail
	errInvalidPCRInfo
	errNoSpace
	errNoSRK
	errNotSealedBlob
	errOwnerSet
	errResources
	errShortRandom
	errSize
	errWrongPCRVal
	errBadParamSize
	errSHAThread
	errSHAError
	errFailedSelfTest
	errAuth2Fail
	errBadTag
	errIOError
	errEncryptError
	errDecryptError
	errInvalidAuthHandle
	errNoEndorsement
	errInvalidKeyUsage
	errWrongEntityType
	errInvalidPostInit
	errInappropriateSig
	errBadKeyProperty
	errBadMigration
	errBadScheme
	errBadDatasize
	errBadMode
	errBadPresence
	errBadVersion
	errNoWrapTransport
	errAuditFailUnsuccessful
	errAuditFailSuccessful
	errNotResettable
	errNotLocal
	errBadType
	errInvalidResource
	errNotFIPS
	errInvalidFamily
	errNoNVPermission
	errRequiresSign
	errKeyNotSupported
	errAuthConflict
	errAreaLocked
	errBadLocality
	errReadOnly
	errPerNoWrite
	errFamilyCount
	errWriteLocked
	errBadAttributes
	errInvalidStructure
	errKeyOwnerControl
	errBadCounter
	errNotFullWrite
	errContextGap
	errMaxNVWrites
	errNoOperator
	errResourceMissing
	errDelegateLock
	errDelegateFamily
	errDelegateAdmin
	errTransportNotExclusive
	errOwnerControl
	errDAAResources
	errDAAInputData0
	errDAAInputData1
	errDAAIssuerSettings
	errDAASettings
	errDAAState
	errDAAIssuerValidity
	errDAAWrongW
	errBadHandle
	errBadDelegate
	errBadContext
	errTooManyContexts
	errMATicketSignature
	errMADestination
	errMASource
	errMAAuthority
)

// Extra messages the TPM might return.
const errDefendLockRunning tpmError = 2051

// tpmErrMsgs maps tpmError codes to their associated error strings.
var tpmErrMsgs = map[tpmError]string{
	errAuthFail:              "authentication failed",
	errBadIndex:              "the index to a PCR, DIR or other register is incorrect",
	errBadParameter:          "one or more parameter is bad",
	errAuditFailure:          "an operation completed successfully but the auditing of that operation failed",
	errClearDisabled:         "the clear disable flag is set and all clear operations now require physical access",
	errDeactivated:           "the TPM is deactivated",
	errDisabled:              "the TPM is disabled",
	errDisabledCmd:           "the target command has been disabled",
	errFail:                  "the operation failed",
	errBadOrdinal:            "the ordinal was unknown or inconsistent",
	errInstallDisabled:       "the ability to install an owner is disabled",
	errInvalidKeyHandle:      "the key handle can not be interpreted",
	errKeyNotFound:           "the key handle points to an invalid key",
	errInappropriateEnc:      "unacceptable encryption scheme",
	errMigrateFail:           "migration authorization failed",
	errInvalidPCRInfo:        "PCR information could not be interpreted",
	errNoSpace:               "no room to load key",
	errNoSRK:                 "there is no SRK set",
	errNotSealedBlob:         "an encrypted blob is invalid or was not created by this TPM",
	errOwnerSet:              "there is already an Owner",
	errResources:             "the TPM has insufficient internal resources to perform the requested action",
	errShortRandom:           "a random string was too short",
	errSize:                  "the TPM does not have the space to perform the operation",
	errWrongPCRVal:           "the named PCR value does not match the current PCR value",
	errBadParamSize:          "the paramSize argument to the command has the incorrect value",
	errSHAThread:             "there is no existing SHA-1 thread",
	errSHAError:              "the calculation is unable to proceed because the existing SHA-1 thread has already encountered an error",
	errFailedSelfTest:        "self-test has failed and the TPM has shutdown",
	errAuth2Fail:             "the authorization for the second key in a 2 key function failed authorization",
	errBadTag:                "the tag value sent to for a command is invalid",
	errIOError:               "an IO error occurred transmitting information to the TPM",
	errEncryptError:          "the encryption process had a problem",
	errDecryptError:          "the decryption process had a problem",
	errInvalidAuthHandle:     "an invalid handle was used",
	errNoEndorsement:         "the TPM does not have an EK installed",
	errInvalidKeyUsage:       "the usage of a key is not allowed",
	errWrongEntityType:       "the submitted entity type is not allowed",
	errInvalidPostInit:       "the command was received in the wrong sequence relative to Init and a subsequent Startup",
	errInappropriateSig:      "signed data cannot include additional DER information",
	errBadKeyProperty:        "the key properties in KEY_PARAMs are not supported by this TPM",
	errBadMigration:          "the migration properties of this key are incorrect",
	errBadScheme:             "the signature or encryption scheme for this key is incorrect or not permitted in this situation",
	errBadDatasize:           "the size of the data (or blob) parameter is bad or inconsistent with the referenced key",
	errBadMode:               "a mode parameter is bad, such as capArea or subCapArea for GetCapability, physicalPresence parameter for PhysicalPresence, or migrationType for CreateMigrationBlob",
	errBadPresence:           "either the physicalPresence or physicalPresenceLock bits have the wrong value",
	errBadVersion:            "the TPM cannot perform this version of the capability",
	errNoWrapTransport:       "the TPM does not allow for wrapped transport sessions",
	errAuditFailUnsuccessful: "TPM audit construction failed and the underlying command was returning a failure code also",
	errAuditFailSuccessful:   "TPM audit construction failed and the underlying command was returning success",
	errNotResettable:         "attempt to reset a PCR register that does not have the resettable attribute",
	errNotLocal:              "attempt to reset a PCR register that requires locality and locality modifier not part of command transport",
	errBadType:               "make identity blob not properly typed",
	errInvalidResource:       "when saving context identified resource type does not match actual resource",
	errNotFIPS:               "the TPM is attempting to execute a command only available when in FIPS mode",
	errInvalidFamily:         "the command is attempting to use an invalid family ID",
	errNoNVPermission:        "the permission to manipulate the NV storage is not available",
	errRequiresSign:          "the operation requires a signed command",
	errKeyNotSupported:       "wrong operation to load an NV key",
	errAuthConflict:          "NV_LoadKey blob requires both owner and blob authorization",
	errAreaLocked:            "the NV area is locked and not writeable",
	errBadLocality:           "the locality is incorrect for the attempted operation",
	errReadOnly:              "the NV area is read only and can't be written to",
	errPerNoWrite:            "there is no protection on the write to the NV area",
	errFamilyCount:           "the family count value does not match",
	errWriteLocked:           "the NV area has already been written to",
	errBadAttributes:         "the NV area attributes conflict",
	errInvalidStructure:      "the structure tag and version are invalid or inconsistent",
	errKeyOwnerControl:       "the key is under control of the TPM Owner and can only be evicted by the TPM Owner",
	errBadCounter:            "the counter handle is incorrect",
	errNotFullWrite:          "the write is not a complete write of the area",
	errContextGap:            "the gap between saved context counts is too large",
	errMaxNVWrites:           "the maximum number of NV writes without an owner has been exceeded",
	errNoOperator:            "no operator AuthData value is set",
	errResourceMissing:       "the resource pointed to by context is not loaded",
	errDelegateLock:          "the delegate administration is locked",
	errDelegateFamily:        "attempt to manage a family other than the delegated family",
	errDelegateAdmin:         "delegation table management not enabled",
	errTransportNotExclusive: "there was a command executed outside of an exclusive transport session",
	errOwnerControl:          "attempt to context save a owner evict controlled key",
	errDAAResources:          "the DAA command has no resources available to execute the command",
	errDAAInputData0:         "the consistency check on DAA parameter inputData0 has failed",
	errDAAInputData1:         "the consistency check on DAA parameter inputData1 has failed",
	errDAAIssuerSettings:     "the consistency check on DAA_issuerSettings has failed",
	errDAASettings:           "the consistency check on DAA_tpmSpecific has failed",
	errDAAState:              "the atomic process indicated by the submitted DAA command is not the expected process",
	errDAAIssuerValidity:     "the issuer's validity check has detected an inconsistency",
	errDAAWrongW:             "the consistency check on w has failed",
	errBadHandle:             "the handle is incorrect",
	errBadDelegate:           "delegation is not correct",
	errBadContext:            "the context blob is invalid",
	errTooManyContexts:       "too many contexts held by the TPM",
	errMATicketSignature:     "migration authority signature validation failure",
	errMADestination:         "migration destination not authenticated",
	errMASource:              "migration source incorrect",
	errMAAuthority:           "incorrect migration authority",
	errDefendLockRunning:     "the TPM is defending against dictionary attacks and is in some time-out period",
}
