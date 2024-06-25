package gen

import (
	"github.com/urfave/cli/v2"
)

var (
	flags = []cli.Flag{
		&cli.StringFlag{
			Name:     "config",
			Aliases:  []string{"c"},
			Usage:    "The path to generate config",
			Required: true,
		},
	}

	Command = &cli.Command{
		Name:   "gen",
		Usage:  "Generate example server code",
		Flags:  flags,
		Action: action,
	}
)

func action(c *cli.Context) error {
	// if utils.Exists(app.ConfigFilePath) {
	// 	fmt.Printf("Config has exist. >>> %s\n", app.ConfigFilePath)
	// 	return nil
	// }
	// example, err := getEmbedFile("config.yaml.example")
	// if err != nil {
	// 	fmt.Println(err)
	// 	return err
	// }
	// if err := ioutil.WriteFile(app.ConfigFilePath, example, os.ModePerm); err != nil {
	// 	fmt.Println(err)
	// 	return err
	// }

	// fmt.Printf("Generate config success! >>> %s\n", app.ConfigFilePath)
	return nil
}
