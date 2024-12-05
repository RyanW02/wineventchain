package validate

import (
	"fmt"
	"github.com/manifoldco/promptui"
	"os"
	"regexp"
)

// MinLength returns a ValidateFunc that checks if the input is at least min characters long, inclusive of min.
func MinLength(min int) promptui.ValidateFunc {
	return func(input string) error {
		if len(input) < min {
			return fmt.Errorf("input must be at least %d characters long", min)
		}

		return nil
	}
}

// LengthBetween returns a ValidateFunc that checks if the input is between min and max characters long, inclusive
// of min and max.
func LengthBetween(min, max int) promptui.ValidateFunc {
	return func(input string) error {
		if len(input) < min {
			return fmt.Errorf("input must be at least %d characters long", min)
		} else if len(input) > max {
			return fmt.Errorf("input must be at most %d characters long", max)
		}

		return nil
	}
}

var sha256Regex = regexp.MustCompile(`^[a-f0-9]{64}$`)

func Sha256Hash(input string) error {
	if !sha256Regex.MatchString(input) {
		return fmt.Errorf("input must be a SHA256 hash")
	}

	return nil
}

func FileExists(input string) error {
	info, err := os.Stat(input)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist")
		} else {
			return err
		}
	}

	if info.IsDir() {
		return fmt.Errorf("file is a directory")
	}

	return nil
}
