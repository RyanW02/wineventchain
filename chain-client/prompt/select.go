package prompt

import (
	"errors"
	"fmt"
	"github.com/manifoldco/promptui"
)

type SelectOption struct {
	Label string
	Emoji string
	Func  func() error
}

func (o SelectOption) String() string {
	if len(o.Emoji) > 0 {
		return o.Emoji + " " + o.Label
	} else {
		return o.Label
	}
}

func NewSelectOption(label, emoji string, f func() error) SelectOption {
	return SelectOption{
		Label: label,
		Emoji: emoji,
		Func:  f,
	}
}

func Select[T fmt.Stringer](label string, options ...T) (*T, error) {
	return SelectWithConfig(label, nil, options...)
}

func SelectWithConfig[T fmt.Stringer](label string, configurator func(p *promptui.Select), options ...T) (*T, error) {
	prompt := promptui.Select{
		Label:        label,
		Items:        options,
		HideSelected: true,
	}

	if configurator != nil {
		configurator(&prompt)
	}

	i, _, err := prompt.Run()
	if err != nil {
		return nil, err
	}

	if i < 0 || i >= len(options) {
		return nil, errors.New("i was less than len(options)")
	}

	return &options[i], err
}

func SelectString(label string, options ...string) (string, error) {
	prompt := promptui.Select{
		Label:        label,
		Items:        options,
		HideSelected: true,
	}

	_, res, err := prompt.Run()
	return res, err
}

func SelectAndExecute(label string, options ...SelectOption) error {
	option, err := Select(label, options...)
	if err != nil {
		return err
	}

	return option.Func()
}

func SelectAndExecuteWithConfig(label string, configurator func(p *promptui.Select), options ...SelectOption) error {
	option, err := SelectWithConfig(label, configurator, options...)
	if err != nil {
		return err
	}

	return option.Func()
}
