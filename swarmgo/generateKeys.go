/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package swarmgo

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	gc "github.com/untillpro/gochips"
	"io/ioutil"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

func generatePrivateKey(bitSize int) (*rsa.PrivateKey, error) {
	// Private Key generation
	privateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, err
	}

	// Validate Private Key
	err = privateKey.Validate()
	if err != nil {
		return nil, err
	}

	gc.Info("Private Key generated")
	return privateKey, nil
}

// encodePrivateKeyToPEMAndEncrypt encodes Private Key from RSA to PEM format
func encodePrivateKeyToPEMAndEncrypt(privateKey *rsa.PrivateKey, password string) []byte {
	// Get ASN.1 DER format
	privDER := x509.MarshalPKCS1PrivateKey(privateKey)

	// Encrypt DER key to encrypted PEM block
	privatePEM, err := x509.EncryptPEMBlock(rand.Reader, "RSA PRIVATE KEY", privDER, []byte(password),
		x509.PEMCipherDES)
	CheckErr(err)

	return pem.EncodeToMemory(privatePEM)
}

// generatePublicKey take a rsa.PublicKey and return bytes suitable for writing to .pub file
// returns in the format "ssh-rsa ..."
func generatePublicKey(privateKey *rsa.PublicKey) []byte {
	publicRsaKey, err := ssh.NewPublicKey(privateKey)
	CheckErr(err)

	pubKeyBytes := ssh.MarshalAuthorizedKey(publicRsaKey)

	gc.Info("Public key generated")
	return pubKeyBytes
}

// writePemToFile writes keys to a file
func writeKeyToFile(keyBytes []byte, saveFileTo string) error {
	folder := filepath.Dir(saveFileTo)
	err := os.MkdirAll(folder, os.ModePerm)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(saveFileTo, keyBytes, 0600)
	if err != nil {
		return err
	}
	gc.Info(fmt.Sprintf("Key saved to: %s", saveFileTo))
	return nil
}

// generate keys and write them to file, can return error
func generateKeysAndWriteToFile(bitSize int, privateKeyFile, publicKeyFile, password string) error {
	privateKey, err := generatePrivateKey(bitSize)
	if err != nil {
		gc.Fatal(err.Error())
		return err
	}

	publicKeyBytes := generatePublicKey(&privateKey.PublicKey)

	privateKeyBytes := encodePrivateKeyToPEMAndEncrypt(privateKey, password)

	err = writeKeyToFile(privateKeyBytes, privateKeyFile)
	if err != nil {
		gc.Fatal(err.Error())
		return err
	}

	err = writeKeyToFile([]byte(publicKeyBytes), publicKeyFile)
	if err != nil {
		gc.Fatal(err.Error())
		return err
	}

	return nil
}
