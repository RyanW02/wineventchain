package prompt

import (
	"github.com/manifoldco/promptui"
)

func Text(label string, validators ...promptui.ValidateFunc) (string, error) {
	p := promptui.Prompt{
		Label:       label,
		Validate:    combineValidators(validators...),
		HideEntered: true,
	}

	return p.Run()
}

func TextWithDefault(label, defaultValue string, validators ...promptui.ValidateFunc) (string, error) {
	p := promptui.Prompt{
		Label:       label,
		Default:     defaultValue,
		AllowEdit:   true,
		Validate:    combineValidators(validators...),
		HideEntered: true,
	}

	return p.Run()
}

func combineValidators(validators ...promptui.ValidateFunc) promptui.ValidateFunc {
	return func(s string) error {
		for _, validator := range validators {
			if err := validator(s); err != nil {
				return err
			}
		}

		return nil
	}
}
