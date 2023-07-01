//go:build dev

package cfg

import (
	"fmt"
	"path/filepath"
)

func (c *Configuration) printConfig() {
	configName := filepath.Base(c.path)
	fmt.Printf("##################### Load %s begin #####################\n", configName)
	c.ko.Print()
	fmt.Printf("#####################  Load %s end  #####################\n", configName)
}
