package dangerous

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"hash"
	"time"
)

var (
	blank_bytes = []byte("")
	DefaultSep  = "."
)

type Signature interface {
	GetSignature(key, value []byte) []byte
	VerifySignature(key, value, sig []byte) bool
}

type SigningAlgorithm struct {
}

func (sa SigningAlgorithm) GetSignature(key, value []byte) []byte {
	return []byte{} // panic if empty
}
func (sa SigningAlgorithm) VerifySignature(key, value, sig []byte) bool {
	return ByteCompare(sig, sa.GetSignature(key, value))
}

type HMACAlgorithm struct {
	DigestMethod func() hash.Hash
}

func (ha HMACAlgorithm) GetSignature(key, value []byte) []byte {
	_hmac := hmac.New(ha.DigestMethod, key)
	_hmac.Write(value)
	return _hmac.Sum(nil)
}

func (ha HMACAlgorithm) VerifySignature(key, value, sig []byte) bool {
	return ByteCompare(sig, ha.GetSignature(key, value))
}

type Signer struct {
	Secret        string
	Salt          string
	Sep           string
	SecretBytes   []byte
	SaltBytes     []byte
	SepBytes      []byte
	KeyDerivation string // concat, django-concat, hmac
	DigestMethod  func() hash.Hash
	Algorithm     Signature // HMACAlgorithm, NoneAlgorithm
}

func (self *Signer) SetDefault() {
	if self.Secret == "" {
		panic("Signer secret is empty.")
	}
	self.SecretBytes = WantBytes(self.Secret)
	if self.Salt == "" {
		self.Salt = "itsdangerous.Signer"
	}
	if self.Sep == "" {
		self.Sep = DefaultSep
	}
	self.SepBytes = WantBytes(self.Sep)
	self.SaltBytes = WantBytes(self.Salt)

	if bytes.Contains(Base64_alphabet, self.SepBytes) {
		fmt.Println(
			"The given separator cannot be used because it may be" +
				" contained in the signature itself. Alphanumeric" +
				" characters and `-_=` must not be used. Now we set Sep to DefaultSep(.)")
	}
	if self.KeyDerivation == "" {
		self.KeyDerivation = "django-concat"
	}
	if self.DigestMethod == nil {
		self.DigestMethod = sha256.New
	}
	if !IsValidStruct(self.Algorithm) {
		self.Algorithm = HMACAlgorithm{DigestMethod: self.DigestMethod}
	}
}

func IsValidStruct(t interface{}) bool {
	switch t.(type) {

	case HMACAlgorithm:
		return true

	case SigningAlgorithm:
		return true

	default:
		return false
	}
}

func (self *Signer) DeriveKey() ([]byte, error) {

	if self.KeyDerivation == "concat" {
		msg, _ := Concentrate(self.SaltBytes, self.SecretBytes)
		funcs := self.DigestMethod()
		funcs.Write(msg)
		return funcs.Sum(nil), nil

	} else if self.KeyDerivation == "django-concat" {
		msg, _ := Concentrate(self.SaltBytes, []byte("signer"))
		msg, _ = Concentrate(msg, self.SecretBytes)
		funcs := self.DigestMethod()
		funcs.Write(msg)
		return funcs.Sum(nil), nil

	} else if self.KeyDerivation == "hmac" {
		mac := hmac.New(self.DigestMethod, self.SecretBytes)
		mac.Write(self.SaltBytes)
		return mac.Sum(nil), nil

	} else if self.KeyDerivation == "none" {
		return self.SecretBytes, nil

	}
	return []byte("Error"), fmt.Errorf("Unknown key derivation method")
}

func (self Signer) GetSignature(value []byte) []byte {
	key, err := self.DeriveKey()
	if err != nil {
		panic(fmt.Sprintf("Signer.GetSignature: %s.", err))
	}
	sig := self.Algorithm.(Signature).GetSignature(key, value)
	return WantBytes(B64encode(sig))
}

func (self Signer) Sign(value string) []byte {
	(&self).SetDefault()
	value_b := WantBytes(value)
	msg, _ := Concentrate(value_b, self.SepBytes)
	msg, _ = Concentrate(msg, self.GetSignature(value_b))
	return msg
}

func (self Signer) VerifySignature(value []byte, sig []byte) bool {
	(&self).SetDefault()
	key, err := self.DeriveKey()
	if err != nil {
		return false
	}
	sigb, err := B64decode(sig)
	if err != nil {
		return false
	}
	return self.Algorithm.(Signature).VerifySignature(key, value, sigb)
}

func (self Signer) UnSign(signed_values string) ([]byte, error) {
	(&self).SetDefault()
	signed_value := WantBytes(signed_values)
	sep := self.SepBytes
	if !bytes.Contains(signed_value, sep) {
		return blank_bytes, fmt.Errorf("BadSignature: No %s found in value", self.Sep)
	}
	value, sig := RSplit(signed_value, sep)
	if self.VerifySignature(value, sig) {
		return value, nil
	}
	return blank_bytes, fmt.Errorf("BadSignature: Signature %s does not match. Value: %s", sig, value)
}

func (self Signer) Validate(signed_value string) bool {
	_, err := self.UnSign(signed_value)
	if err != nil {
		return false
	}
	return true
}

func (self Signer) get_timestamp() int64 {
	return time.Now().UTC().Unix()
}

func (self Signer) SignTimestamp(values string) []byte {
	(&self).SetDefault()
	value := WantBytes(values)
	timestamp := WantBytes(B64encode(Int2Bytes(self.get_timestamp())))
	sep := WantBytes(self.Sep)
	value, _ = Concentrate(value, sep, timestamp)
	value, _ = Concentrate(value, sep, self.GetSignature(value))
	return value
}

func (self Signer) UnSignTimestamp(values string, max_age int64) ([]byte, int64, error) {
	(&self).SetDefault()
	result, err := self.UnSign(values)
	if err != nil {
		return result, 0, err
	}
	sep := WantBytes(self.Sep)
	if !bytes.Contains(result, sep) {
		return result, 0, fmt.Errorf("BadTimeSignature-timestamp missing")
	}
	value, ts := RSplit(result, sep)
	decode, err := B64decode(ts)
	if err != nil {
		return value, 0, fmt.Errorf("BadTimeSignature-%s", err)
	}
	timestamp := Bytes2Int(decode)
	if err != nil {
		return value, timestamp, fmt.Errorf("BadTimeSignature-Malformed timestamp")
	}
	if max_age >= 0 {
		age := self.get_timestamp() - timestamp
		if age > max_age {
			return value, timestamp, fmt.Errorf("SignatureExpired-Signature age %d > %d seconds", age, max_age)
		}
	}
	return value, timestamp, nil
}

func (self Signer) ValidateTimestamp(signed_value string, max_age int64) bool {
	(&self).SetDefault()
	_, _, err := self.UnSignTimestamp(signed_value, max_age)
	if err != nil {
		return false
	}
	return true

}
