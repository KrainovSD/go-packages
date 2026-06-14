package helpers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"io"
)

type CryptP256Options struct {
	PrivateKey        string
	ExternalPublicKey string
}
type CryptP256 struct {
	privateKey        []byte
	externalPublicKey []byte
}

func CreateP256PublicKeyFromPrivate(privateKey string) (string, error) {
	var privateKeyBytes []byte
	var err error
	if privateKeyBytes, err = base64.StdEncoding.DecodeString(privateKey); err != nil {
		return "", fmt.Errorf("decode private key: %w", err)
	}
	var curve = ecdh.P256()
	var key *ecdh.PrivateKey
	if key, err = curve.NewPrivateKey(privateKeyBytes); err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key.PublicKey().Bytes()), nil
}

func CreateCryptP256(o CryptP256Options) (*CryptP256, error) {
	var err error
	var privateKeyBytes []byte
	var externalPublicKeyBytes []byte

	if privateKeyBytes, err = base64.StdEncoding.DecodeString(o.PrivateKey); err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}
	if externalPublicKeyBytes, err = base64.StdEncoding.DecodeString(o.ExternalPublicKey); err != nil {
		return nil, fmt.Errorf("decode external public key: %w", err)
	}

	return &CryptP256{
		privateKey:        privateKeyBytes,
		externalPublicKey: externalPublicKeyBytes,
	}, nil
}

func EncryptP256(content []byte, publicKey []byte) ([]byte, error) {
	var err error
	var curve = ecdh.P256()
	var externalPublicKey *ecdh.PublicKey
	if externalPublicKey, err = curve.NewPublicKey(publicKey); err != nil {
		return nil, fmt.Errorf("parse external public key: %w", err)
	}
	var ephemeralPrivateKey *ecdh.PrivateKey
	if ephemeralPrivateKey, err = curve.GenerateKey(rand.Reader); err != nil {
		return nil, fmt.Errorf("generate ephemeral key: %w", err)
	}
	var ephemeralPublicKey = ephemeralPrivateKey.PublicKey()
	var sharedSecret []byte
	if sharedSecret, err = ephemeralPrivateKey.ECDH(externalPublicKey); err != nil {
		return nil, fmt.Errorf("get shared ECDH secret: %w", err)
	}
	var salt = make([]byte, 32)
	if _, err = rand.Read(salt); err != nil {
		return nil, fmt.Errorf("gen salt: %w", err)
	}
	var hkdfReader = hkdfNew(sha256.New, sharedSecret, salt, nil)
	var aesKey = make([]byte, 32)
	if _, err = io.ReadFull(hkdfReader, aesKey); err != nil {
		return nil, fmt.Errorf("gen aes key: %w", err)
	}
	var block cipher.Block
	if block, err = aes.NewCipher(aesKey); err != nil {
		return nil, fmt.Errorf("get new cipher block: %w", err)
	}
	var aesgcm cipher.AEAD
	if aesgcm, err = cipher.NewGCM(block); err != nil {
		return nil, fmt.Errorf("newGCM: %w", err)
	}
	var nonce = make([]byte, aesgcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("gen nonce: %w", err)
	}
	var cipherContent = aesgcm.Seal(nil, nonce, content, nil)
	var cipherInfo = make([]byte, 0, 65+32+12+len(cipherContent))
	cipherInfo = append(cipherInfo, ephemeralPublicKey.Bytes()...)
	cipherInfo = append(cipherInfo, salt...)
	cipherInfo = append(cipherInfo, nonce...)
	cipherInfo = append(cipherInfo, cipherContent...)

	return cipherInfo, nil
}

func EncryptedBytesP256ToString(encrypted []byte) string {
	return base64.StdEncoding.EncodeToString(encrypted)
}

func EncryptedStringP256ToBytes(encrypted string) ([]byte, error) {
	var decoded []byte
	var err error
	if decoded, err = base64.StdEncoding.DecodeString(encrypted); err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}
	return decoded, nil
}

func DecryptP256(content []byte, privateKey []byte) ([]byte, error) {
	var err error
	var decrypted []byte
	var externalEphemeralPublicBytes = content[:65]
	var salt = content[65:97]
	var nonce = content[97:109]
	var encrypted = content[109:]
	var curve = ecdh.P256()
	var key *ecdh.PrivateKey
	if key, err = curve.NewPrivateKey(privateKey); err != nil {
		return decrypted, fmt.Errorf("parse private key: %w", err)
	}
	var externalEphemeralPublicKey *ecdh.PublicKey
	if externalEphemeralPublicKey, err = curve.NewPublicKey(externalEphemeralPublicBytes); err != nil {
		return decrypted, fmt.Errorf("parse external ephemeral public key: %w", err)
	}
	var sharedSecret []byte
	if sharedSecret, err = key.ECDH(externalEphemeralPublicKey); err != nil {
		return decrypted, fmt.Errorf("get shared secret: %w", err)
	}
	var hkdfReader = hkdfNew(sha256.New, sharedSecret, salt, nil)
	var aesKey = make([]byte, 32)
	if _, err = io.ReadFull(hkdfReader, aesKey); err != nil {
		return decrypted, fmt.Errorf("gen aes key: %w", err)
	}
	var block cipher.Block
	if block, err = aes.NewCipher(aesKey); err != nil {
		return decrypted, fmt.Errorf("get new cipher block: %w", err)
	}
	var aesgcm cipher.AEAD
	if aesgcm, err = cipher.NewGCM(block); err != nil {
		return decrypted, fmt.Errorf("newGCM: %w", err)
	}
	if decrypted, err = aesgcm.Open(nil, nonce, encrypted, nil); err != nil {
		return decrypted, fmt.Errorf("decode aes: %w", err)
	}
	return decrypted, nil
}

func (c *CryptP256) EncryptP256(content []byte) ([]byte, error) {
	return EncryptP256(content, c.externalPublicKey)
}

func (c *CryptP256) DecryptP256(content []byte) ([]byte, error) {
	return DecryptP256(content, c.privateKey)
}

func KeyGenP256() ([]byte, []byte, error) {
	var err error
	var curve = ecdh.P256()
	var privateKey *ecdh.PrivateKey
	var privateBytes []byte
	var publicBytes []byte

	if privateKey, err = curve.GenerateKey(rand.Reader); err != nil {
		return privateBytes, publicBytes, fmt.Errorf("gen private key: %w", err)
	}

	privateBytes = privateKey.Bytes()
	publicBytes = privateKey.PublicKey().Bytes()

	return privateBytes, publicBytes, nil
}
func KeyGenP256String() (string, string, error) {
	var err error
	var private string
	var public string
	var privateBytes []byte
	var publicBytes []byte

	if privateBytes, publicBytes, err = KeyGenP256(); err != nil {
		return private, public, fmt.Errorf("get keys: %w", err)
	}

	private = base64.StdEncoding.EncodeToString(privateBytes)
	public = base64.StdEncoding.EncodeToString(publicBytes)

	return private, public, err
}

/** FROM OFFICIAL GO PACKAGE */
type hkdf struct {
	expander hash.Hash
	size     int

	info    []byte
	counter byte

	prev []byte
	buf  []byte
}

func (f *hkdf) Read(p []byte) (int, error) {
	need := len(p)
	remains := len(f.buf) + int(255-f.counter+1)*f.size
	if remains < need {
		return 0, errors.New("hkdf: entropy limit reached")
	}
	n := copy(p, f.buf)
	p = p[n:]

	for len(p) > 0 {
		if f.counter > 1 {
			f.expander.Reset()
		}
		f.expander.Write(f.prev)
		f.expander.Write(f.info)
		f.expander.Write([]byte{f.counter})
		f.prev = f.expander.Sum(f.prev[:0])
		f.counter++

		f.buf = f.prev
		n = copy(p, f.buf)
		p = p[n:]
	}
	f.buf = f.buf[n:]

	return need, nil
}

func extract(hash func() hash.Hash, secret, salt []byte) []byte {
	if salt == nil {
		salt = make([]byte, hash().Size())
	}
	extractor := hmac.New(hash, salt)
	extractor.Write(secret)
	return extractor.Sum(nil)
}

func expand(hash func() hash.Hash, pseudorandomKey, info []byte) io.Reader {
	expander := hmac.New(hash, pseudorandomKey)
	return &hkdf{expander, expander.Size(), info, 1, nil, nil}
}

func hkdfNew(hash func() hash.Hash, secret, salt, info []byte) io.Reader {
	prk := extract(hash, secret, salt)
	return expand(hash, prk, info)
}
