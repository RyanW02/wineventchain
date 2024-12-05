package prompt

import (
	"encoding/json"
	"github.com/manifoldco/promptui"
)

func Display(label, message string) error {
	p := promptui.Select{
		Label:        label,
		Items:        []string{"⬅️ Back"},
		HideSelected: true,
		Templates: &promptui.SelectTemplates{
			Details: message,
		},
	}

	_, _, err := p.Run()
	return err
}

func DisplayMarshalled(label string, obj interface{}) error {
	marshalled, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	return Display(label, string(marshalled))
}
