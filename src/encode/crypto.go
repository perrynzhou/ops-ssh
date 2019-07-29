package encode

import (
	"crypto/aes"
	"crypto/cipher"
)

var commonIV = []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}

var commonKey = "abcdefg@2019$0123456789!@#$%^*&("

func Encoding(b []byte) ([]byte, error) {
	c, err := aes.NewCipher([]byte(commonKey))
	if err != nil {
		return nil, err
	}
	// encoding string
	cfb := cipher.NewCFBEncrypter(c, commonIV)
	ciphertext := make([]byte, len(b))
	cfb.XORKeyStream(ciphertext, b)
	return ciphertext, nil
}
func Decoding(b []byte) ([]byte, error) {
	c, err := aes.NewCipher([]byte(commonKey))
	if err != nil {
		return nil, err
	}
	cfbdec := cipher.NewCFBDecrypter(c, commonIV)
	originData := make([]byte, len(b))
	cfbdec.XORKeyStream(originData, b)
	return originData, nil
}
