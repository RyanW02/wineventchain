package internal

import (
	"github.com/RyanW02/wineventchain/chain-client/prompt"
	"github.com/manifoldco/promptui"
	"os"
)

func (c *Client) OpenMainMenu() error {
	return prompt.SelectAndExecute("Choose an option",
		prompt.NewSelectOption("Interact with node", "📲", c.OpenAppSelector),
		prompt.NewSelectOption("Select private key", "🔑", c.OpenPrivateKeySelector),
		prompt.NewSelectOption("Quit", "❌ ", func() error { // Extra space in emoji name needed
			os.Exit(0)
			return nil
		}),
	)
}

func (c *Client) OpenPrivateKeySelector() error {
	options := make([]prompt.SelectOption, 0, len(c.Config.Client.PrivateKeyFiles)+2)

	// Add import option
	options = append(options, prompt.NewSelectOption("Import", "📥", c.HandleImportPrivateKey))

	activeIndex := 0
	for username, key := range c.Config.Client.PrivateKeyFiles {
		username, key := username, key
		options = append(options, prompt.NewSelectOption(username, "🔑", func() error {
			c.Config.Client.ActivePrivateKey = &username
			c.ActivePrincipal = &username
			if err := c.Config.Write(); err != nil {
				return err
			}

			// Load private key from disk
			privKey, err := LoadPrivateKey(key)
			if err != nil {
				return err
			}

			c.ActivePrivateKey = privKey

			return c.OpenMainMenu()
		}))

		if c.ActivePrincipal != nil && *c.ActivePrincipal == username {
			activeIndex = len(options) - 1
		}
	}

	// Add back option
	options = append(options, prompt.NewSelectOption("Back", "⬅️", c.OpenMainMenu))

	return prompt.SelectAndExecuteWithConfig("Choose a private key", func(p *promptui.Select) {
		p.CursorPos = activeIndex
	}, options...)
}

func (c *Client) OpenAppSelector() error {
	return prompt.SelectAndExecute("Choose an app",
		prompt.NewSelectOption("Identity", "\U0001FAAA", c.OpenIdentityActionSelector),
		prompt.NewSelectOption("Events", "📝", c.OpenEventsActionSelector),
		prompt.NewSelectOption("Event Retention Policy", "🧲", c.OpenRetentionPolicyActionSelector),
		prompt.NewSelectOption("ABCI Info", "ℹ️", c.HandleABCIInfo),
		prompt.NewSelectOption("Back", "⬅️", c.OpenMainMenu),
	)
}

func (c *Client) OpenIdentityActionSelector() error {
	return prompt.SelectAndExecute("Choose an action",
		prompt.NewSelectOption("Seed", "🌱", c.HandleSeed),
		prompt.NewSelectOption("Register", "👤", c.HandleRegister),
		prompt.NewSelectOption("View Principal", "🔍", c.HandleViewPrincipal),
		prompt.NewSelectOption("Back", "⬅️", c.OpenAppSelector),
	)
}

func (c *Client) OpenEventsActionSelector() error {
	return prompt.SelectAndExecute("Choose an action",
		prompt.NewSelectOption("Create", "➕ ", c.HandleCreateEvent),
		prompt.NewSelectOption("View", "🔍", c.HandleViewEvent),
		prompt.NewSelectOption("Back", "⬅️", c.OpenAppSelector),
	)
}

func (c *Client) OpenRetentionPolicyActionSelector() error {
	return prompt.SelectAndExecute("Choose an action",
		prompt.NewSelectOption("Deploy Policy File", "📦", c.HandlePolicyDeploy),
		prompt.NewSelectOption("View Active Policy", "🔍", c.HandlePolicyView),
		prompt.NewSelectOption("Back", "⬅️", c.OpenAppSelector),
	)
}
