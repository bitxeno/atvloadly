package app

import "github.com/bitxeno/atvloadly/internal/log"

var bootLauncher = &Launcher{}

type Bootstrapper interface {
	// Name() string
	Boot() error
}

type BootFunc func() error

func (bf BootFunc) Boot() error {
	return bf()
}

type Launcher struct {
	Boots []Bootstrapper
}

func (l *Launcher) Add(boots ...Bootstrapper) {
	l.Boots = append(l.Boots, boots...)
}

func (l *Launcher) Prepend(boots ...Bootstrapper) {
	l.Boots = append(boots, l.Boots...)
}

func (l *Launcher) Run() {
	for _, boot := range l.Boots {
		err := boot.Boot()
		if err != nil {
			log.Panic(err.Error())
		}
	}
}

func addBoots(boots ...Bootstrapper) {
	bootLauncher.Boots = append(bootLauncher.Boots, boots...)
}

func addBoot(fn BootFunc) {
	bootLauncher.Boots = append(bootLauncher.Boots, BootFunc(fn))
}
