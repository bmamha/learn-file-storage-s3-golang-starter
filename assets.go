package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func generateAssetName(media_type string) string {
	random_bytes := make([]byte, 32)
	rand.Read(random_bytes)
	random_name := base64.RawURLEncoding.EncodeToString(random_bytes)
	extension := mediaTypeToExt(media_type)
	video_name := fmt.Sprintf("%s.%s", random_name, extension)
	return video_name
}

func mediaTypeToExt(mediaType string) string {
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return ".bin"
	}
	return parts[1]
}
