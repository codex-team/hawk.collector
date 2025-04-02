package errorshandler

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"strings"
)

func decompressGzipString(gzipString []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(gzipString))
	if err != nil {
		return []byte(""), fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.Close()

	var result bytes.Buffer
	_, err = io.Copy(&result, reader)
	if err != nil {
		return []byte(""), fmt.Errorf("failed to decompress data: %w", err)
	}

	return result.Bytes(), nil
}

func getSentryKeyFromAuth(auth string) (string, error) {
	auth = strings.TrimPrefix(auth, "Sentry ")
	pairs := strings.Split(auth, ", ")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 && kv[0] == "sentry_key" {
			return kv[1], nil
		}
	}

	log.Infof("Sentry key not found in auth header: %s", auth)

	return "", errors.New("sentry_key not found")
}
