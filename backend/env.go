package main

import (
	"bufio"
	"log"
	"os"
	"strings"
)

// loadDotEnv reads simple KEY=VALUE lines from the first file that exists among
// paths and puts them in the environment. Real environment variables always
// win, so `DATABASE_URL=... go run .` overrides the file.
//
// Supports `# comments`, blank lines, an optional `export ` prefix, and values
// wrapped in single or double quotes.
func loadDotEnv(paths ...string) {
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			line = strings.TrimPrefix(line, "export ")
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if len(value) >= 2 {
				if (value[0] == '"' && value[len(value)-1] == '"') ||
					(value[0] == '\'' && value[len(value)-1] == '\'') {
					value = value[1 : len(value)-1]
				}
			}
			if key == "" {
				continue
			}
			if _, exists := os.LookupEnv(key); exists {
				continue
			}
			os.Setenv(key, value)
		}
		if err := scanner.Err(); err != nil {
			log.Printf("read %s: %v", path, err)
			return
		}
		log.Printf("loaded env from %s", path)
		return
	}
}
