package cryptoutil

// Package cryptoutil implementa criptografia RSA compatível com a lógica Java:
//   Encrypt: URLEncode → RSA/PKCS1v15 (chunks) → Base64
//   Decrypt: Base64    → RSA/PKCS1v15 (chunks) → URLDecode

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// LoadPublicKey carrega uma *rsa.PublicKey a partir de um caminho de arquivo PEM
// ou de uma string PEM inline (detectado pelo prefixo "-----BEGIN").
// Suporta os formatos PKIX (SubjectPublicKeyInfo) e PKCS1.
func LoadPublicKey(source string) (*rsa.PublicKey, error) {
	data, err := readPEM(source)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("cryptoutil: bloco PEM inválido na chave pública")
	}

	// Tenta PKIX (formato mais comum para chaves públicas — "PUBLIC KEY")
	if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		rsaKey, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("cryptoutil: chave pública PKIX não é RSA")
		}
		return rsaKey, nil
	}

	// Fallback: PKCS1 ("RSA PUBLIC KEY")
	return x509.ParsePKCS1PublicKey(block.Bytes)
}

// LoadPrivateKey carrega uma *rsa.PrivateKey a partir de um caminho de arquivo PEM
// ou de uma string PEM inline. Suporta PKCS8 e PKCS1.
func LoadPrivateKey(source string) (*rsa.PrivateKey, error) {
	data, err := readPEM(source)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("cryptoutil: bloco PEM inválido na chave privada")
	}

	// Tenta PKCS8 primeiro ("PRIVATE KEY") — formato mais moderno
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("cryptoutil: chave privada PKCS8 não é RSA")
		}
		return rsaKey, nil
	}

	// Fallback: PKCS1 ("RSA PRIVATE KEY") — formato legado
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// Encrypt replica exatamente a sequência Java:
//  1. URLEncoder.encode(rawJson, UTF-8)   → url.QueryEscape
//  2. RSA/PKCS1v15 encrypt em chunks      → processChunksEncrypt
//  3. Base64.getEncoder().encodeToString  → base64.StdEncoding
func Encrypt(plainText string, pubKey *rsa.PublicKey) (string, error) {
	encoded := url.QueryEscape(plainText)
	data := []byte(encoded)

	// Tamanho máximo por chunk para PKCS1v15: keySize - 11
	// (equivale ao ENCRYPT_CHUNK_SIZE do Java)
	chunkSize := pubKey.Size() - 11

	encrypted, err := processChunksEncrypt(data, chunkSize, pubKey)
	if err != nil {
		return "", fmt.Errorf("cryptoutil: erro ao criptografar: %w", err)
	}

	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// Decrypt replica exatamente a sequência Java:
//  1. Base64.getDecoder().decode           → base64.StdEncoding
//  2. RSA/PKCS1v15 decrypt em chunks       → processChunksDecrypt
//  3. URLDecoder.decode(result, UTF-8)     → url.QueryUnescape
func Decrypt(cipherText string, privKey *rsa.PrivateKey) (string, error) {
	encryptedBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cipherText))
	if err != nil {
		return "", fmt.Errorf("cryptoutil: base64 inválido: %w", err)
	}

	// Tamanho de cada bloco cifrado = tamanho da chave em bytes
	// (equivale ao DECRYPT_CHUNK_SIZE do Java)
	chunkSize := privKey.Size()

	decrypted, err := processChunksDecrypt(encryptedBytes, chunkSize, privKey)
	if err != nil {
		return "", fmt.Errorf("cryptoutil: erro ao descriptografar: %w", err)
	}

	plainText, err := url.QueryUnescape(string(decrypted))
	if err != nil {
		return "", fmt.Errorf("cryptoutil: erro ao decodificar URL: %w", err)
	}

	return plainText, nil
}

// PrettyJSON formata uma string JSON com indentação de 2 espaços.
// Retorna a string original sem modificação se o input não for JSON válido.
func PrettyJSON(raw string) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(raw), "", "  "); err != nil {
		return raw
	}
	return buf.String()
}

// processChunksEncrypt replica o processInChunks do Java no modo ENCRYPT_MODE.
// Divide os dados em blocos de chunkSize bytes e criptografa cada bloco separadamente,
// concatenando os resultados — necessário porque RSA tem limite de bytes por operação.
func processChunksEncrypt(data []byte, chunkSize int, key *rsa.PublicKey) ([]byte, error) {
	var buf bytes.Buffer
	for offset := 0; offset < len(data); {
		end := offset + chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunk, err := rsa.EncryptPKCS1v15(rand.Reader, key, data[offset:end])
		if err != nil {
			return nil, err
		}
		buf.Write(chunk)
		offset = end
	}
	return buf.Bytes(), nil
}

// processChunksDecrypt replica o processInChunks do Java no modo DECRYPT_MODE.
// Cada bloco cifrado tem exatamente chunkSize bytes (= tamanho da chave RSA).
func processChunksDecrypt(data []byte, chunkSize int, key *rsa.PrivateKey) ([]byte, error) {
	var buf bytes.Buffer
	for offset := 0; offset < len(data); {
		end := offset + chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunk, err := rsa.DecryptPKCS1v15(rand.Reader, key, data[offset:end])
		if err != nil {
			return nil, err
		}
		buf.Write(chunk)
		offset = end
	}
	return buf.Bytes(), nil
}

// readPEM detecta se source é uma string PEM inline ou um caminho de arquivo
// e retorna os bytes do PEM correspondente.
func readPEM(source string) ([]byte, error) {
	if strings.HasPrefix(strings.TrimSpace(source), "-----") {
		return []byte(source), nil
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, fmt.Errorf("cryptoutil: não foi possível ler %s: %w", source, err)
	}
	return data, nil
}
