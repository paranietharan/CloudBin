package env

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"
)

func LoadFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}

		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("warning: failed to scan %s: %v", path, err)
	}
}

func String(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func Int(key string, fallback int) int {
	parsed, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return parsed
}

func Bool(key string, fallback bool) bool {
	parsed, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return parsed
}
