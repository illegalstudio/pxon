package knownhosts

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var foundLinePattern = regexp.MustCompile(`found: line ([0-9]+)`)

type Matches struct {
	Hosts []string
	Rows  int
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	return filepath.Join(home, ".ssh", "known_hosts"), nil
}

func Find(path string, hosts []string) (Matches, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Matches{}, nil
		}
		return Matches{}, fmt.Errorf("inspect SSH known hosts file %s: %w", path, err)
	}

	sshKeygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		return Matches{}, fmt.Errorf("ssh-keygen command not found: %w", err)
	}

	var matches Matches
	matchedRows := make(map[int]struct{})
	fallbackRows := 0

	for _, host := range uniqueHosts(hosts) {
		output, err := exec.Command(sshKeygen, "-F", host, "-f", path).CombinedOutput()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
				continue
			}
			return Matches{}, fmt.Errorf("search SSH known hosts for %s: %s: %w", host, strings.TrimSpace(string(output)), err)
		}

		rows, fallback := matchedLineNumbers(output)
		if len(rows) == 0 && fallback == 0 {
			continue
		}

		matches.Hosts = append(matches.Hosts, host)
		for _, row := range rows {
			matchedRows[row] = struct{}{}
		}
		fallbackRows += fallback
	}

	matches.Rows = len(matchedRows) + fallbackRows
	return matches, nil
}

func Remove(path string, hosts []string) error {
	sshKeygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		return fmt.Errorf("ssh-keygen command not found: %w", err)
	}

	for _, host := range uniqueHosts(hosts) {
		output, err := exec.Command(sshKeygen, "-R", host, "-f", path).CombinedOutput()
		if err != nil {
			return fmt.Errorf("remove SSH known host %s: %s: %w", host, strings.TrimSpace(string(output)), err)
		}
	}

	return nil
}

func uniqueHosts(hosts []string) []string {
	seen := make(map[string]struct{}, len(hosts))
	result := make([]string, 0, len(hosts))

	for _, host := range hosts {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}

		key := strings.ToLower(host)
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		result = append(result, host)
	}

	return result
}

func matchedLineNumbers(output []byte) ([]int, int) {
	var rows []int
	fallbackRows := 0
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#") {
			match := foundLinePattern.FindStringSubmatch(line)
			if len(match) != 2 {
				continue
			}

			row, err := strconv.Atoi(match[1])
			if err == nil {
				rows = append(rows, row)
			}
			continue
		}

		if len(rows) == 0 {
			fallbackRows++
		}
	}

	return rows, fallbackRows
}
